# ADR 0003 — Publicaciones: descubrimiento automático de productos "listos" + encolado

- **Estado:** aceptado — implementación COMPLETA en código (2026-09-08); falta ejecutar el DDL del índice único a mano
- **Fecha:** 2026-09-08
- **Servicio:** `services/core-orchestrator`
- **Origen:** hoy no hay ningún mecanismo que publique automáticamente un producto
  nuevo en los marketplaces. `ecom_channel_sync_queue` solo se llena a mano
  (`POST /api/channel-sync-queue`) o como efecto secundario de un `CreateListings`
  que falló su precondición. El `connection_id` lo elige quien llama, y a veces
  manda uno incorrecto.

## Contexto

### Cómo se publica y se refresca hoy

- **Crear listing:** `POST /api/channel-listings/publish` →
  `channel_listings.Service.CreateListings` → `publishReady` → `publisher.Publish`
  (MercadoLibre / Odoo). Solo crea; una `(product, connection, fitment)` que ya
  tiene fila en `ecom_channel_product_map` se saltea (salvo `status = 'closed'`).
- **Refrescar precio/stock:** `POST /api/channel-listings/refresh` →
  `RefreshListings`. **Ya es automático**: al terminar cada sync de stock
  ERP/Nissan (`sync.completed` en RabbitMQ), `refreshListingsAfterErpStockSync` /
  `refreshListingsAfterNissanSync` ([sync/service.go:167](../../services/core-orchestrator/internal/application/sync/service.go),
  [:272](../../services/core-orchestrator/internal/application/sync/service.go))
  llaman `RefreshListings` para un conjunto de `connectionId` hardcodeado.
- **Worker actual (`workers.MarketplaceWorker`):** hace polling de
  `ecom_channel_sync_queue` y procesa dos tipos de fila:
  - `sync_type = 'listing'` → `channel_listings.RetryQueuedEntry` (re-verifica
    stock + imagen, re-publica).
  - cualquier otro `sync_type` → `ProductSyncer` registrado por canal. **Solo
    está registrado `ODOO`**; una fila `full` para una conexión MercadoLibre se
    queda `pending` para siempre porque no hay syncer.

### Por qué "elegir conexión por categoría mapeada" no basta

Ni MercadoLibre ni Odoo exigen que `ecom_channel_category_map` tenga fila previa:

- **MercadoLibre** ([resolveExternalCategoryID](../../services/core-orchestrator/internal/application/sync/sync_mercadolibre_products.go)):
  si hay mapeo lo usa; si no, llama al **predictor de categorías de MELI** y usa
  la mejor. El mapeo se graba *después* de publicar.
- **Odoo** ([resolveOdooCategory](../../services/core-orchestrator/internal/application/sync/sync_odoo_products.go)):
  si no hay mapeo, **crea la categoría en Odoo al vuelo** y luego hace `Upsert`.

Gatear el descubrimiento por "categoría ya mapeada" tiene un problema
huevo-y-gallina (el mapeo aparece recién tras la primera publicación) y dejaría a
Odoo fuera. **Decisión de negocio explícita:** aun así se adopta ese gate como
*control operativo* — un producto solo se auto-publica en una conexión cuya
categoría fue mapeada a mano previamente. Es una restricción nueva y deliberada,
no un reflejo de cómo publica el sistema.

## Decisión

Se reemplaza `workers.MarketplaceWorker` por **dos procesos independientes**, con
`ecom_channel_sync_queue` como única frontera entre ellos.

### 1. Scheduler de descubrimiento (productor) — 1 vez al día

Nuevo `schedulers.ListingDiscoveryScheduler`, mismo patrón que
`CompatibilitiesFixScheduler` (corre a una hora fija, misma zona horaria que el
resto de los servicios, bajo `safe.Supervise` + `safe.Do` por corrida).
Cadencia: **una vez al día, a la 01:00**.

**Paso 1 — productos "listos".** Una sola query paginada sobre `ecom_products`,
todos los gates como `EXISTS`/`JOIN` (sin N+1). Un producto está listo si:

| Gate | Regla |
|---|---|
| No borrado | `p.deleted_at IS NULL` |
| Activo y vendible | `p.status = 'active'` y `p.is_sellable = 1` |
| Categoría | `p.category_id IS NOT NULL` |
| Marca | `p.brand_id IS NOT NULL` |
| Stock | `SUM(ecom_product_stock.available_qty) > 0` (suma de todas las filas, igual que `channel_listings.sumAvailableStock`) |
| Imagen | existe `ecom_product_images` con `is_first = 1` |
| Precio | **precio efectivo > 0** (ver abajo) |

**Precio.** Se resuelve exactamente como en `publish` hoy:
`pricing.EffectivePriceResolver.ResolveInCurrency(productID, targetCurrencyID)` →
toma la lista de precios de mayor prioridad (`FindEffectivePrice`), convierte vía
`ecom_exchange_rates` si la moneda difiere, y se le aplica
`PricingFormulaCalculator.CalculatePrice`. `targetCurrencyID` es la moneda de la
conexión destino (`ecom_channel_connections.currency_id`). Debe existir un precio
(`ErrProductPriceNotFound` → no listo) **y el resultado debe ser > 0** (gate
nuevo: hoy `publish` no rechaza un precio `0.00`, solo la ausencia de fila).

Como el precio depende de la moneda de la conexión, este gate se evalúa por
`(producto, conexión)` en el paso 3, no en el paso 1.

**Paso 2 — conexiones destino.** Para cada producto listo, las conexiones donde
su `category_id` está mapeada:

```sql
SELECT ccm.connection_id
FROM ecom_channel_category_map ccm
JOIN ecom_channel_connections cc ON cc.id = ccm.connection_id
WHERE ccm.category_id = :category_id
  AND ccm.deleted_at IS NULL
  AND cc.deleted_at IS NULL
  AND cc.status = 'active'
```

**Paso 3 — encolar.** Por cada `(producto, conexión)` se inserta una fila en
`ecom_channel_sync_queue` (`sync_type = 'listing'`, `updated_by` = actor de
sistema) **solo si**:

- el precio efectivo en la moneda de esa conexión existe y es > 0 (gate de
  precio, arriba);
- **no hay ninguna publicación del producto en esa conexión**:
  `NOT EXISTS (ecom_channel_product_map WHERE product_id = :pid AND connection_id
  = :cid AND deleted_at IS NULL AND status <> 'closed')`. Cualquier fila cuenta —
  no se mira `vehicle_fitment_id`. Un producto con al menos un listing en la
  conexión **nunca** se vuelve a encolar por este scheduler;
- no hay ya una fila en cola reutilizable para ese par (ver dedupe).

### 2. Fan-out de fitments: fuera de alcance del scheduler

El scheduler solo publica productos **sin ninguna publicación** en la conexión.
En conexiones con `allows_multiple_listings = true`, `publishReady` hace el
fan-out inicial (un listing por compatibilidad *pooled* de la cadena de
sucesión). Las compatibilidades **agregadas después** de esa primera publicación
no las toca el scheduler: se publican **manualmente** desde la vista de detalle
del producto ("Sincronización" → `product_sync`, que ya calcula los fitments
`pending`). Trade-off aceptado.

### 3. Una fila por par: reactivar en vez de re-insertar

Hay **como mucho una fila** en `ecom_channel_sync_queue` por
`(product_id, connection_id, sync_type = 'listing')` en toda su vida. El scheduler,
por cada par que decidió encolar, mira si ya existe una:

| Fila previa | Acción del scheduler |
|---|---|
| ninguna | `INSERT` una fila `pending` |
| `status IN ('pending', 'processing')` | no hace nada (ya está en curso) |
| `status = 'done'` | no hace nada (ya publicado; además el chequeo de `ecom_channel_product_map` ya lo habría filtrado) |
| `status = 'failed'` y `attempts < 5` | **reactiva la misma fila**: `UPDATE SET status = 'pending', updated_by = <sistema>` — **no** toca `attempts` ni `last_error` |
| `status = 'failed'` y `attempts >= 5` | **no hace nada** — la fila queda `failed` a la espera de intervención manual (visible en el admin-dashboard) |

`attempts` queda entonces como **contador de fallos acumulado de por vida** para
ese par: cada vez que el consumer falla, `MarkFailed` hace `attempts = attempts + 1`
(comportamiento actual, sin cambios). Al día siguiente el scheduler vuelve a
poner la fila en `pending` y, si vuelve a fallar, `attempts` sigue subiendo —
**hasta 5**. A partir de `attempts >= 5` el scheduler deja de reactivarla: para
que se reintente, alguien tiene que corregir el dato y resetear la fila a
`pending` a mano (o el futuro endpoint del admin-dashboard). `last_error`
siempre refleja el **último** error. `maxListingQueueAttempts = 5` como
constante.

**Índice único.** Se agrega un índice único sobre
`(product_id, connection_id, sync_type)` acotado a filas no borradas (columna
generada + unique, ver el DDL al final del ADR), para que la invariante "una
fila por par" no dependa solo del código. El scheduler igual hace el chequeo
aplicativo (para decidir INSERT vs UPDATE); el índice es la red de seguridad
ante una carrera.

### 4. Consumer de la cola (publicador) — cada 30 min

El `MarketplaceWorker` actual se **simplifica** a un solo camino:

- Se elimina la rama `ProductSyncer` (`processEntry` → `syncer.Sync`), el mapa
  `syncers`, el método `Register` y el registro `marketplaceWorker.Register("ODOO", …)`.
- `OdooProductSyncService.Sync` queda sin usar → se elimina (su `Publish` /
  `Refresh` siguen, los usa `channel_listings`).
- El worker hace polling de `ecom_channel_sync_queue` (`status = 'pending'`,
  `sync_type = 'listing'`) cada **30 min**, claim atómico por fila
  (`Claim` → `processing`), y publica.

**Publicación.** Se reutiliza la lógica actual de `POST /api/channel-listings/publish`.
`channel_listings.RetryQueuedEntry(productID, connectionID)` se renombra a
`PublishQueuedProduct`.

El descubridor ya validó stock, precio, imagen, categoría y marca antes de
encolar. El consumer **solo re-valida el stock en vivo**: entre la corrida del
scheduler y el procesamiento de la fila (hasta ~30 min) puede haber ocurrido una
venta que baje el stock a 0. Precio, imagen, categoría y marca no cambian de
forma realista en esa ventana; si cambiaran, la publicación fallaría y la fila
cae en `failed` como cualquier otro error.

- `sumAvailableStock(productID) <= 0` → `ErrChannelListingsNotReady` →
  `ReleasePending` (vuelve a `pending` con nota en `last_error`, **sin** contar
  intento). La fila se vuelve a intentar en cada poll (chequeo de stock = un
  `SUM` barato, sin llamada externa) hasta que el producto se reponga y se
  publique.
- El resto (imagen/categoría/marca/precio) **no se re-chequea**.

**Éxito / fallo.** `PublishQueuedProduct` no hace fan-out a la cadena de
sucesión (solo publica el producto de la fila); `publishReady` sí decide entre
listing general y uno por fitment según `allows_multiple_listings` y la excepción
Nissan. Al terminar:

- todo OK → `MarkDone`.
- **cualquier error de `Publish`** → `MarkFailed`: `status = 'failed'`,
  `attempts = attempts + 1`, el mensaje completo en `last_error`. **No importa la
  causa** — atributos required faltantes que MELI rechazó, token, timeout, dato
  inválido: todos se registran igual. Por eso el consumer **no** valida que los
  atributos del producto estén completos; publica y deja que el error (si lo
  hay) caiga en `last_error`.
- Al día siguiente el scheduler encuentra la fila `failed` y la reactiva a
  `pending` (§3); si vuelve a fallar, `attempts` incrementa otra vez.

**`publish` (el endpoint `POST /api/channel-listings/publish`) no se toca.** Su
comportamiento actual —crear el ítem y devolver `missingRequiredAttributes` como
dato informativo cuando MELI lo acepta incompleto, o propagar el error cuando
MELI lo rechaza— se deja como está. La cola solo se apoya en el segundo caso:
si hubo error, va a `failed` + `last_error`.

### 5. Excepción Nissan: no se duplica

La regla "marca Nissan (`ecom_brands.id = 1`) → un solo listing general aunque
`allows_multiple_listings = true`" ya vive en `channel_listings.publishReady` y
en `sync_mercadolibre_products.Upload`. El scheduler y el consumer **no la
replican**: encolan `(producto, conexión)` y `publishReady` decide. El
`nissanBrandID = 1` hardcodeado se deja como está (sacarlo a config es otro ADR).

### 6. Se elimina `POST /api/channel-sync-queue`

El endpoint, su handler (`ChannelSyncQueueHandler`) y el módulo
`internal/application/channel_sync_queue` se eliminan. La necesidad de "forzar la
publicación de un SKU desde el admin-dashboard" se cubrirá con un endpoint nuevo
dedicado (fuera del alcance de este ADR) que encole `sync_type = 'listing'`
pasando por los mismos gates.

### 7. `CreateListings` deja de encolar

`channel_listings.Service.queuePending` se elimina. Cuando `processChainMember`
detecta que un producto no tiene stock o no tiene imagen de portada usable,
`CreateListings` ya **no** inserta una fila en `ecom_channel_sync_queue`: solo
devuelve el motivo en `ListingOutcome.Error` (`reasonNoStock` / `reasonNoImage`).
El scheduler diario recogerá ese producto por su cuenta en cuanto cumpla las
condiciones. Los campos `ListingOutcome.Queued` y `ListingOutcome.QueueID`
quedan sin uso → se eliminan del struct y de la respuesta del endpoint.

Con esto, el **único** productor de filas `ecom_channel_sync_queue` es el
scheduler de descubrimiento (§1). El consumer (§4) es el único que las mueve de
estado.

## Flujo resultante

```
# Diario (scheduler descubrimiento)
scan ecom_products (gates: stock>0, precio efectivo>0, imagen is_first, categoría, marca)
  → por producto: conexiones con category_id mapeada y activa
    → si no hay publicación (ecom_channel_product_map, status<>closed):
        sin fila / done              → INSERT ecom_channel_sync_queue (sync_type='listing', pending)
        fila failed, attempts < 5    → UPDATE status='pending' (misma fila, no toca attempts)
        fila failed, attempts >= 5   → no hace nada (espera intervención manual)
        fila pending/processing      → no hace nada

# Cada 30 min (consumer)
FindPending(sync_type='listing') → Claim → PublishQueuedProduct
  re-valida SOLO stock en vivo (una venta pudo bajarlo a 0 en la ventana de ~30 min)
    stock 0   → ReleasePending (last_error, no cuenta intento; reintenta cada poll)
    error     → MarkFailed (status='failed', attempts++, last_error = mensaje completo)
    ok        → MarkDone   (publishReady: general o por fitment; Nissan = general)

# Automático post-RabbitMQ (sin cambios)
sync.completed → RefreshListings(connectionId)  # precio/stock a listings ya publicados
```

## Puntos abiertos

1. **Conjunto hardcodeado de conexiones del refresh.**
   `hardcodedErpStockRefreshConnectionIDs` / `hardcodedNissanRefreshConnectionIDs`
   siguen hardcodeados. Podrían derivarse de las conexiones activas con
   listings, pero es otro cambio.
2. **Config nueva:** `ListingDiscoveryRunAtHour/Minute` (default 01:00), y el
   `SyncQueuePollInterval` pasa a 30 min (revisar `cfg` actual).
4. **Endpoint de reemplazo de `POST /api/channel-sync-queue`** para el
   admin-dashboard (encolar `sync_type = 'listing'` a mano por SKU/conexión, o
   resetear una fila `failed` con `attempts >= 5`). Fuera del alcance de este ADR.

## Consecuencias

- **+** Un producto nuevo que cumple todas las condiciones se publica solo, en
  las conexiones correctas, sin que nadie elija el `connection_id` a mano.
- **+** Descubrir y publicar quedan desacoplados: el barrido pesado corre 1×/día;
  publicar (APIs externas, lento, con reintento) corre cada 30 min con auditoría
  en la fila (`attempts`, `last_error`, `status`).
- **+** El precio/stock sigue refrescándose solo tras cada sync de RabbitMQ; este
  ADR no lo toca.
- **−** El gate "categoría mapeada" obliga a pre-mapear categorías por conexión a
  mano antes de que algo se auto-publique (control deliberado, pero es trabajo
  operativo y bloquea Odoo hasta que se mapee).
- **−** Los fitments agregados a un producto ya publicado no se auto-publican;
  requieren acción manual desde la vista de detalle.
- **−** Un par `(producto, conexión)` que falla 5 veces queda `failed` y no se
  reintenta solo: requiere que alguien corrija el dato y resetee la fila.
- **−** Un producto encolado que pierde stock antes de publicarse deja su fila
  `pending` reintentándose indefinidamente (barato: solo un `SUM`) hasta que se
  reponga.
- **−** `POST /api/channel-sync-queue` desaparece; el frontend que lo use hoy se
  rompe hasta que exista el endpoint de reemplazo.

## DDL — índice único (ejecutar manualmente)

`ecom_channel_sync_queue` no tiene forma nativa de "unique parcial" en MySQL 8,
así que se usa una columna generada que vale `1` para filas vivas y `NULL` para
borradas (los `NULL` no colisionan en un índice único):

```sql
-- 1) Pre-chequeo: ¿hay pares duplicados vivos que impedirían crear el índice?
SELECT product_id, connection_id, sync_type, COUNT(*) AS filas
FROM ecom_channel_sync_queue
WHERE deleted_at IS NULL
GROUP BY product_id, connection_id, sync_type
HAVING COUNT(*) > 1;

-- 2) Si el paso 1 devuelve filas, resolver dejando solo la más reciente por par
--    (revisar antes de correr; ajusta si preferís conservar otra):
UPDATE ecom_channel_sync_queue q
JOIN (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY product_id, connection_id, sync_type
               ORDER BY id DESC
           ) AS rn
    FROM ecom_channel_sync_queue
    WHERE deleted_at IS NULL
) d ON d.id = q.id
SET q.deleted_at = CURRENT_TIMESTAMP
WHERE d.rn > 1;

-- 3) Columna generada + índice único
ALTER TABLE ecom_channel_sync_queue
    ADD COLUMN active_row TINYINT UNSIGNED
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,
    ADD UNIQUE KEY uq_sync_queue_active_pair
        (product_id, connection_id, sync_type, active_row);
```

Con el índice, el scheduler puede encolar con
`INSERT ... ON DUPLICATE KEY UPDATE status = 'pending', updated_by = VALUES(updated_by)`
y dejar que la BD resuelva la carrera, aunque igual conviene el chequeo previo
para respetar el tope de `attempts` (que `ON DUPLICATE KEY` no puede condicionar).

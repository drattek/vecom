# ADR 0002 — Stock: reconciliación por ausencia + limpieza de Redis

- **Estado:** aceptado — implementación COMPLETA (2026-09-05)
- **Fecha:** 2026-09-05
- **Servicio:** `services/core-orchestrator`
- **Origen:** stock local que nunca bajaba a 0 cuando un almacén se quedaba sin existencias

## Contexto

`synapse-bridge` lee las fuentes de stock filtrando `stock > 0`:

- Nissan: `RE_VEXISTENCIAS WHERE RELA_EXISTENCIAACTUAL > 0` (~104k filas; un SKU
  aparece una vez por agencia/almacén, y de 8 agencias normalmente solo 2 tienen
  existencia).
- ERP general: `dyn.ItemInventLocation WHERE Disponible > 0`.

El filtro es deliberado: sin él se leerían decenas de miles de filas en cero y el
sync (que ya tardaba horas antes de las optimizaciones de la fase **sync**) sería
inviable.

**Problema:** el feed filtrado es un *snapshot parcial no autoritativo*. Cuando el
stock de `almacén-1` pasa de 1 a 0, su fila simplemente deja de venir. "La fila no
vino" es indistinguible de "no cambió", así que:

- `syncProductStock` nunca recibe el 0 y no lo escribe (además tiene el guard "no
  escribir si no cambió").
- Peor: `RedisProductService` en el bridge solo hace `SET` (sin TTL, sin `DEL`),
  así que la clave `nissan:existencia:<sku>-<agencia>` queda con el último valor
  positivo para siempre y `ScanExistencias` la sigue re-sincronizando.

Resultado: stock local colgado en un valor > 0 que ya no existe en el ERP, y que
se propaga a los marketplaces.

## Decisión

### 1. Reconciliación por ausencia (mark-and-sweep) en `core-orchestrator`

Cada corrida de sync de stock se trata como snapshot autoritativo de "todo lo que
tiene stock > 0" **de esa fuente**. Durante la corrida se acumula en memoria
(`stockSeen`, `sync_stock_reconcile.go`):

- el set de claves `(product_id, branch_id, warehouse_id)` que vinieron en el feed
  (`mark` se llama antes de escribir: si la fuente lo reportó > 0, no se baja a 0
  aunque el write falle);
- el set de `warehouse_id` vistos al menos una vez.

Al terminar de escribir el feed y **antes** del refresh de listings,
`reconcileZeroedStock`:

1. lista `ecom_product_stock` con `available_qty > 0` join `ecom_products` por
   `source_id` (`FindPositiveBySource`);
2. por cada fila: si su almacén **no** apareció en la corrida → **skip** (se asume
   falta de cobertura de esa corrida, no un vaciado real); si la clave apareció →
   skip; en otro caso → baja a 0;
3. la baja es `UPDATE ecom_product_stock SET available_qty = 0` + `INSERT
   ecom_stock_movements` (`movement_type = 'sync'`, `quantity_change` negativo),
   en lotes de 250 compartiendo transacción (`WithinTx`, mismo criterio que
   `nissanSyncBatchSize`).

Es lógica de negocio → vive en `internal/application/sync`, no en el bridge (que
sigue read-only y sin lógica de producto). Es idempotente: las filas ya en 0 no
vuelven a salir de `FindPositiveBySource`, así que un reintento del consumer no
duplica movimientos. Un error se loguea y **no** se propaga (devolverlo haría que
RabbitMQ reencole y reprocese todo el sync; la próxima corrida reintenta igual).

### 2. Alcance por `source_id`

El barrido se acota a `ecom_products.source_id` (NISSAN o DYNAMICS). Un sync de
Nissan nunca toca stock del ERP general y viceversa. `erpStockSyncCache` ahora
resuelve la fuente DYNAMICS (`findOrCreateDynamicsSource`), igual que
`nissanSyncCache` ya resolvía NISSAN.

### 3. Salvaguardas: "almacén visto" + umbral por almacén

Dos salvaguardas contra vaciados espurios por un feed truncado:

- **Almacén visto al menos una vez.** Solo se confía en los ceros por ausencia
  dentro de un almacén que apareció en la corrida. Si un almacén entero no trae
  ninguna fila, se saltea (no se vacía).
- **Umbral por almacén (`stockReconcileMaxZeroRatio = 0.5`).** Si en un almacén
  visto más del 50% de sus posiciones con stock > 0 intentaran bajar a 0, se omite
  bajar a 0 **ese almacén entero** y se loguea. Un vaciado > 50% de un día para el
  otro es más probable que sea un problema de la fuente (página truncada, timeout)
  que stock real. El umbral es por almacén, no global: un almacén chico con
  problemas no bloquea la reconciliación de los demás.

Implicación: un almacén con muy pocas posiciones (p. ej. 1 sola) que se vacíe de
verdad no se reconcilia hasta que vuelva a aparecer al menos una posición suya —
mismo trade-off aceptado que la regla del almacén visto.

### 4. Limpieza de Redis tras consumir

`NissanRepository.DeleteKeys` / `InventoryRepository.DeleteKeys` borran, al final
del consumer, la **lista exacta** de claves escaneadas y consumidas (no un `SCAN`
+ `DEL` por patrón — eso pisaría una corrida del bridge en curso). Así las claves
que dejan de venir no quedan como fantasmas. Un error al borrar solo se loguea: la
baja de stock ya la resolvió el paso 1.

## Orden en el consumer

`stock.sync.completed` / `nissan.existencias.sync.completed`:

```
scan claves Redis → MGET → escribir feed en MySQL (con stockSeen)
  → reconcileZeroedStock(source_id, seen)
  → DeleteKeys(claves)
  → refresh listings marketplaces
```

## Consecuencias

- **+** El stock local converge al ERP sin depender de una columna de "última
  modificación" en las fuentes.
- **+** Sin volumen extra de lectura en la fuente (los ceros se infieren por
  ausencia, no se leen).
- **+** Redis deja de crecer indefinidamente con claves muertas.
- **−** Un almacén que se queda 100% sin stock (ninguna fila en el feed), o que
  se vacía > 50% en una corrida, no se reconcilia hasta la próxima corrida con
  cobertura normal. Aceptado (es preferible a un vaciado espurio por feed roto).
- **−** `FindPositiveBySource` carga en memoria todas las filas de stock positivo
  de la fuente (~100k filas ≈ pocos MB). Aceptable.
- **−** Si el consumer se reprocesa tras borrar las claves de Redis pero antes de
  `refresh`, la corrida ve el feed vacío; `stockSeen` queda vacío y la
  reconciliación no baja nada (la regla del almacén visto lo protege).

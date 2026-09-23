# ADR 0005 — Selección manual de categoría externa por conexión, antes de publicar

- **Estado:** aceptado — implementación COMPLETA en código (2026-09-17); falta ejecutar el DDL a mano (`infrastructure/mysql/2026-09-17_add_channel_product_category_selection.sql`)
- **Fecha:** 2026-09-17
- **Servicio:** `services/core-orchestrator`, `services/admin-dashboard`
- **Origen:** un mismo `ecom_categories` local (p.ej. "Frenos") puede contener productos que
  MercadoLibre exige publicar bajo categorías externas distintas (disco de freno → categoría ML
  "Frenos", balatas → categoría ML "Balatas"). `sync_mercadolibre_products.resolveExternalCategoryID`
  reutilizaba, como atajo, la categoría externa ya mapeada para la categoría **local** del
  producto (`ecom_channel_category_map`) apenas existía una — así que el primer producto de
  "Frenos" publicado fijaba esa categoría ML para todos los demás productos de "Frenos", sin
  volver a consultar el predictor. El mismo acoplamiento afectaba al checklist de atributos
  requeridos (`product_attributes_checklist`), que resolvía los atributos obligatorios a partir
  de la categoría local del producto en vez de la categoría externa real.

## Contexto

`ecom_products.category_id` es la categoría general del catálogo (un solo eje, compartido por
Odoo y por la organización interna) — no tiene sentido convertirla en multi-valor por conexión.
Lo que hacía falta era desacoplar, solo para el momento de publicar/completar atributos en un
canal categorizado (MercadoLibre), "qué categoría externa corresponde a este producto en esta
conexión" de "cuál es la categoría general del producto".

Piezas ya existentes que este ADR reutiliza tal cual:

- `category_import.Service.Import(ctx, connectionID, externalCategoryID, actorID, onlySelectedConnection=true)`
  ya resuelve/crea, para MercadoLibre (vía `EnsureLocalCategory`) y para Odoo (lógica de
  replicación de path equivalente), una categoría local hoja + fila en `ecom_channel_category_map`
  para una categoría externa dada, **sin depender de ningún producto**.
- `category_import.Service.BrowseTree` (`GET /api/category-import/tree`, ya consumido por
  `ExternalCategoryTree.tsx`) navega el árbol externo de ambos canales.
- `GET /api/marketplaces/mercadolibre/category-predictor` ya sugiere una categoría ML por texto.

Lo que faltaba era: (a) un lugar para guardar, por producto y por conexión, cuál de esas
categorías externas eligió el usuario **antes** de que exista ninguna publicación, y (b) que el
checklist de atributos y la resolución de categoría al publicar prioricen esa elección sobre el
atajo por categoría local.

No se puede guardar esa elección "pendiente de publicar" en `ecom_channel_product_map`:
`channel_listings.publishOne` trata cualquier fila con `status <> 'closed'` como "ya publicado" y
saltea la publicación real — precrear una fila ahí antes de publicar rompería el flujo.

## Decisión

### 1. Tabla nueva `ecom_channel_product_category_selection`

```sql
CREATE TABLE ecom_channel_product_category_selection (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    product_id BIGINT UNSIGNED NOT NULL,
    connection_id BIGINT UNSIGNED NOT NULL,
    category_id BIGINT UNSIGNED NOT NULL,
    external_category_id VARCHAR(255) NOT NULL,
    external_category_name VARCHAR(255) DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    UNIQUE KEY uq_product_connection_category_selection (product_id, connection_id),
    CONSTRAINT fk_pcs_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_pcs_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_pcs_category FOREIGN KEY (category_id) REFERENCES ecom_categories(id)
);
```

`category_id` es la hoja local que `category_import.Import` resolvió/creó para
`external_category_id` — se guarda para no tener que re-derivarla en cada lectura, pero
**nunca** se escribe en `ecom_products.category_id`. Único por (product_id, connection_id):
un producto solo puede tener una elección pendiente por conexión, y volver a elegir antes de
publicar la reemplaza.

### 2. Flujo de UI

**Revisión 2026-09-18:** el selector de categoría vivía originalmente en `ProductSyncSection`
(pestaña Sincronización), duplicado junto al checklist de atributos que ya existía en la pestaña
Atributos. Se movió el selector a Atributos (`AttributesChecklistCard`, junto al `<Select>` de
conexión que esa pestaña ya tenía) para no tener dos lugares distintos resolviendo lo mismo:
Sincronización pasa a ser puramente de estado + acción (no vuelve a mostrar la tabla de
atributos ni el selector).

En Atributos, al elegir una conexión cuyo `checklist.state === "needs_selection"`, en vez del
mensaje de error se muestra el selector en el lugar de la tabla:

- **MercadoLibre** (tiene predictor): se llama primero al predictor con el nombre del producto;
  se muestra la sugerencia con opción de aceptarla o de abrir el árbol de categorías (mismo
  componente `ExternalCategoryTree` de Settings → Categorías) para elegir otra.
- **Odoo** (sin predictor): se muestra directamente el árbol.

Elegir una hoja dispara `POST /api/products/{productId}/details/sync/{connectionId}/category`, que:

1. Llama a `category_import.Service.Import` (reutilizado, `onlySelectedConnection=true`) para
   asegurar la categoría local + `ecom_channel_category_map` de esa categoría externa.
2. Si el canal es de scope `category` (MercadoLibre — ver ADR 0004), llama a
   `channel_attribute_values.ProvisionCategoryAttributes` con la categoría local resuelta en (1)
   como override explícito (ver §3) para sembrar los slots de atributos requeridos de **esa**
   categoría externa, no la del producto.
3. Guarda la elección en `ecom_channel_product_category_selection` (upsert).

En Sincronización, `PublishGate` solo *lee* `useAttributeChecklist` (misma consulta, sin
parámetros nuevos) para decidir: sin `externalCategoryId` → mensaje "elegí la categoría desde
Atributos", sin selector propio; con `externalCategoryId` → nombre de la categoría + "Publicar",
habilitado solo cuando, además, no hay atributos requeridos faltantes
(`attributeScope !== "category" || requiredMissing === 0`) — completar esos atributos también se
hace desde Atributos, no desde Sincronización.

### 3. Override explícito en `ProvisionCategoryAttributesInput`

`ProvisionCategoryAttributes` resolvía la categoría local a la que escalar los slots de
atributos a partir de `product.CategoryID` (si el producto ya tenía una) o de
`EnsureLocalCategory` (si no). Eso es exactamente el acoplamiento que este ADR rompe: se agrega
`LocalCategoryIDOverride *int64` al input — cuando viene seteado, se usa directo como
`localCategoryID` y se saltea por completo la derivación desde `product.CategoryID`. Así los
slots de atributos requeridos de "Balatas" quedan escalados a la categoría local que
`category_import.Import` creó para "Balatas", nunca a la categoría local general del producto
("Frenos").

### 4. `product_attributes_checklist.GetChecklist` prioriza la selección pendiente

**Revisión 2026-09-18:** la primera versión de este ADR conservaba, como último fallback, el
mapeo por categoría local (`general.CategoryID` + `ecom_channel_category_map`) para no romper
productos/conexiones publicados antes de esta funcionalidad. En la práctica esto resucitaba el
bug exacto que el ADR existe para resolver: como `category_import.Import`/`EnsureLocalCategory`
siguen escribiendo en `ecom_channel_category_map` cada vez que alguien elige una categoría
(incluso a través del selector nuevo), apenas el primer producto de una categoría local
compartida (p.ej. "Frenos") tenía una fila ahí, el segundo producto la heredaba directo y el
selector nunca se mostraba — el problema de Frenos/Balatas, otra vez. Se eliminó ese fallback
por completo. `slotCategoryID` se resuelve, en este orden:

1. `ChannelProductCategorySelectionRepository.FindByProductAndConnection` — la elección manual
   (ADR 0005). Si existe: `state = "ok"`.
2. `resolvePublishedCategory` — si el producto **ya tiene una publicación** en esa conexión
   (`ecom_channel_product_map.external_category_id`, por producto, nunca ambiguo), se usa esa
   categoría, revirtiendo el external id a su hoja local vía
   `ChannelCategoryMapRepository.FindByExternalCategoryAndConnection`. Esta sí es una lectura
   segura de `ecom_channel_category_map`: parte de la categoría externa **ya resuelta y
   específica de este producto**, no de la categoría local compartida — es la dirección opuesta
   a la del bug. Cubre los productos/conexiones publicados antes de que existiera el selector.
3. Si ninguna de las dos aplica y el canal es `attribute_scope = "category"`: `"needs_selection"`
   — el front debe mostrar el selector, no la tabla de atributos.

`general.CategoryID`/`ecom_channel_category_map` (leído hacia adelante, desde la categoría del
producto) **nunca** se usan para esta resolución — ni para decidir si hace falta seleccionar, ni
para escalar los slots de atributos.

### 5. Resolución al publicar

- `sync_mercadolibre_products.resolveExternalCategoryID`: consulta la selección pendiente por
  `(product.ID, connectionID)`; si existe, usa su `ExternalCategoryID` directo. **Ya no cae**
  a `ecom_channel_category_map` por categoría local (revisión 2026-09-18, mismo motivo que §4) —
  sin selección, siempre llama al predictor, incluida la publicación automática vía
  `workers.MarketplaceWorker`/`ListingDiscoveryScheduler`, que nunca pasa por el selector manual.
- `sync_odoo_products.resolveOdooCategory` (los dos call-sites que hoy reciben
  `*product.CategoryID`): si hay selección pendiente para `(product.ID, connectionID)`, se pasa
  su `CategoryID` local en su lugar; si no, sigue cayendo a `product.CategoryID` (Odoo no tiene
  el problema de ambigüedad de MercadoLibre — un producto, una categoría — así que ese fallback
  no se tocó).

`ecom_products.category_id` no se lee para ninguna de estas dos resoluciones cuando hay una
selección — deja de ser la fuente de verdad para "qué categoría externa publicar" en cualquier
conexión que tenga su propia elección.

## Cambios en código

- `infrastructure/mysql/eco-architecture.sql` + `2026-09-17_add_channel_product_category_selection.sql`.
- `mysql/channel_product_category_selection_repository.go` (nuevo).
- `application/channel_attribute_values/service.go`: `ProvisionCategoryAttributesInput.LocalCategoryIDOverride`.
- `application/product_category_selection/service.go` (nuevo) + `interfaces/http/product_category_selection_handler.go` (nuevo) + ruta `POST /api/products/{productId}/details/sync/{connectionId}/category`.
- `application/product_attributes_checklist/service.go`: dep `channelProductMap` nueva (fallback por publicación existente, ya no por categoría local), prioridad de resolución, estado `needs_selection`.
- `application/sync/sync_mercadolibre_products.go` (`resolveExternalCategoryID`, sin fallback a `ecom_channel_category_map`) y `sync_odoo_products.go` (`resolveOdooCategory` call-sites vía `resolveConnectionCategoryID`): dep nueva, prioridad de resolución.
- `mysql/channel_connection_repository.go`: `ChannelConnectionDTO.ChannelCode` (join a `ecom_channels.code` en las cuatro consultas existentes) — lo necesita el selector de categoría en Atributos para distinguir MercadoLibre (predictor) de cualquier otro canal (árbol directo), sin adivinar por el nombre.
- `cmd/main.go` / `cmd/mysql_init.go` / `router.go`: wiring.
- `admin-dashboard`: `attributeChecklistSchema` (`needs_selection`), `channelConnectionSchema.channelCode`, hooks `useCategoryPrediction` / `useSelectChannelCategory`, componente compartido `AttributeChecklistTable`. El selector de categoría (`CategorySelector`) vive en `ProductAttributesSection.tsx` (pestaña Atributos, junto al selector de conexión existente) — `ProductSyncSection.tsx` solo lee `useAttributeChecklist` para habilitar "Publicar".

## Consecuencias

- `ecom_categories` y la categoría general del producto quedan intactas — este cambio no les
  quita ninguna función; solo deja de usarlas como fuente de verdad para publicar en un canal
  categorizado.
- Cada conexión de un producto puede terminar apuntando a una categoría externa distinta sin
  contaminar a otros productos de la misma categoría local.
- Un producto/conexión publicado antes de este ADR sigue sin fila en la tabla nueva; el
  checklist y la publicación siguen resolviendo por el camino legado (categoría local +
  `ecom_channel_category_map`) indefinidamente para esos casos — no hay backfill.
- Un producto con múltiples fitments en una conexión `allows_multiple_listings=true` elige la
  categoría una sola vez (por conexión, no por fitment): coherente con que la categoría ML no
  varía por vehículo compatible.

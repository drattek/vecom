# ADR 0004 — Atributos personalizados en Odoo: modelo clave-valor por producto

- **Estado:** aceptado — implementación COMPLETA en código (2026-09-09); falta ejecutar el DDL a mano (`infrastructure/mysql/2026-09-09_add_attribute_scope_to_channels.sql`)
- **Fecha:** 2026-09-09
- **Servicio:** `services/core-orchestrator`
- **Origen:** hasta hoy `OdooProductSyncService` no publica ningún atributo — `Publish` devuelve
  siempre `missingRequiredAttributes = nil` y `create` arma un `product.template` con campos
  fijos (nombre, precio, categoría, imágenes, dimensiones, impuesto, descripción). Todo el
  aparato de `ecom_channel_attributes` / `ecom_channel_attribute_map` / `ecom_product_attributes`
  y el checklist (`product_attributes_checklist`) eran de facto **solo de MercadoLibre**.

## Contexto

### Cómo funcionan los atributos en cada canal

- **MercadoLibre:** el catálogo de atributos lo exige **la categoría**. `ecom_channel_attributes`
  tiene filas con `category_id` puesto y `target_strategy = 'fixed_key'` (llaves conocidas:
  `BRAND`, `MODEL`, `PART_NUMBER`…). `channel_attribute_values.ProvisionCategoryAttributes`
  las siembra leyendo `GET /categories/{id}/attributes` de MELI.
- **Odoo:** los atributos son **por producto, no por categoría**. En este despliegue viven en
  un modelo custom del plugin de Odoo:

  ```python
  class WebsiteSaleProductInfo(models.Model):
      _name = 'website.sale.product.info'
      _order = 'sequence, id'
      sequence = fields.Integer(default=10)
      product_tmpl_id = fields.Many2one('product.template', required=True, ondelete='cascade', index=True)
      info_key = fields.Char(string='Clave', required=True)
      info_value = fields.Char(string='Valor', required=True)
      # unique(product_tmpl_id, info_key)  ← ya implementado en el plugin
  ```

  Es una tabla clave-valor plana colgada de `product.template`. No hay `product.attribute`
  ni variantes ni listas cerradas: `info_value` es texto libre.

### Lo que el esquema ya preveía

`ecom_channel_attributes` tiene dos ejes independientes:

- `target_strategy`: `fixed_key` (ML) vs **`dynamic_field`** ("el canal no tiene llaves fijas,
  el valor se inserta en un campo/mecanismo del modelo" — pensado para Odoo), con `target_field`
  indicando el mecanismo.
- `category_id`: `NULL` = aplica a todas las categorías; no-NULL = solo esa. Que ML lo use por
  categoría es **dato**, no una restricción.

Lo que faltaba era: (a) una forma de que el sistema sepa que un canal es "por producto" para que
el checklist no exija categoría, y (b) que el sync de Odoo lea esos slots y escriba el modelo.

## Decisión

### 1. Columna nueva `ecom_channels.attribute_scope`

```sql
ALTER TABLE ecom_channels
  ADD COLUMN attribute_scope ENUM('category','product') NOT NULL DEFAULT 'category'
  AFTER description;

UPDATE ecom_channels SET attribute_scope = 'product' WHERE code = 'ODOO';
```

- `category` (default): comportamiento actual. El checklist exige categoría asignada **y**
  mapeada al canal antes de mostrar atributos (estados `no_category` / `category_not_mapped`).
- `product`: el checklist **no** exige categoría (estado siempre `ok`) y muestra **solo los
  atributos que ese producto ya tiene con valor** — no la lista completa de slots del canal.
  Los slots `ecom_channel_attributes` de un canal `product` son plumbing compartido (todos con
  `category_id IS NULL`); enumerarlos daría la unión de todo lo que se agregó alguna vez en el
  canal, lista larga e inútil de "completar". Atributos nuevos se dan de alta con el formulario
  (§5); borrar el valor de una fila la saca del checklist. `is_required` no aplica en modo
  `product` (SetValue nunca lo pone; un flag manual en la BD sigue bloqueando la publicación
  como válvula de escape).

Se descartó inferir el scope de "¿todos los slots tienen `category_id NULL`?": es implícito,
frágil (una sola fila con categoría lo rompería) y no expresa intención. Se descartó exponerlo
en el CRUD de canales por ahora — es una propiedad estructural que se fija una vez por canal;
si hace falta editarla desde el admin es un follow-up trivial.

### 2. Configuración de atributos para Odoo

Filas en `ecom_channel_attributes` para el canal `ODOO`:

- `target_strategy = 'dynamic_field'`
- `target_field = 'website.sale.product.info'`
- `external_key` = el `info_key` que se quiere en Odoo (`"Material"`, `"Voltaje"`…)
- `category_id = NULL`
- `value_mode = 'value_name'` siempre (todo va como string; `info_value` es `Char`)
- `is_required = 0` (normalmente): en modo `product` "requerido" no tiene sentido por producto;
  el push a Odoo solo manda las claves que el producto tiene con valor

Filas en `ecom_channel_attribute_map` (`connection_id NULL` = todo el canal):

- `source_type = 'custom_attribute'` + `attribute_id` → `ecom_attributes.id` (el caso normal).
- `source_type = 'static_value'` + `static_value` → clave con valor fijo para todos los
  productos (p.ej. `info_key = 'Garantía'`, `info_value = '12 meses'`).
- `source_type = 'system_field'` → **se ignora** en el push a Odoo: esos datos
  (`brand.name`, `part_number`, `sku`, dimensiones) ya van a campos de `product.template`.
  Mismo criterio que `resolveCustomAttributes` de MELI.

**No** hace falta tabla de mapeo de opciones: sin listas cerradas, un enum local se envía por
su `value` de display.

### 3. Escritura: CRUD directo sobre `website.sale.product.info`

Handler nuevo `infrastructure/marketplace/odoo/handler_product_info.go` sobre la External
JSON-2 API:

- `SearchReadProductInfo(productTmplID)` → filas actuales `{id, info_key, info_value}`.
- `CreateProductInfo(vals_list)`, `WriteProductInfo(id, vals)`, `UnlinkProductInfo(ids)`.

`OdooProductSyncService.syncProductInfo(productTmplID, entries)`:

1. Calcula las entries deseadas (`resolveProductInfoEntries`).
2. Lee las filas actuales del template.
3. Para cada entry deseada: `write` si la clave ya existe con otro valor, `create` si falta,
   nada si ya coincide.
4. `unlink` las filas cuyo `info_key` **está en nuestro universo de claves gestionadas**
   (los `external_key` de los slots del canal) pero ya no está entre las deseadas — p.ej. un
   atributo al que se le borró el valor. Las claves ajenas (añadidas a mano en Odoo) se dejan
   intactas.

Se descartó añadir un `One2many` en `product.template` para escribirlo atómico dentro del
`create`/`write` del producto: implica tocar el plugin y el modelo `website.sale.product.info`
tal cual está no lo declara.

### 4. Gate de requeridos

`OdooProductSyncService.Publish` deja de devolver `nil, nil` y `create` **aborta antes del
`product.template/create`** si un slot marcado `is_required` no tiene valor — mismo mecanismo
que `createNewItem` de MELI. En la práctica, en modo `product` los slots se crean con
`is_required = 0`, así que el gate no dispara: `resolveProductInfoEntries` solo manda a Odoo
las claves que el producto tiene con valor y **no** reporta como faltantes las que no
(son slots compartidos de canal, no huecos del producto). Un `is_required = 1` puesto a mano
sí bloquea. Vía cola (ADR 0003) el error cae en `last_error`; vía
`POST /api/channel-listings/publish` se propaga en la respuesta.

`Refresh` (precio/stock) **no se toca**: los atributos se re-sincronizan en el `update`
completo (ruta de la cola `sync_type` no-listing), igual que la categoría.

### 5. Alta de atributos desde el admin (canales `attribute_scope = 'product'`)

`channel_attribute_values.Service.SetValue` deja de estar hardcodeado a MercadoLibre:
`SetValueInput.ConnectionID` (opcional) elige el canal — `nil` → MercadoLibre (comportamiento
histórico); con conexión → el canal de esa conexión. Para un canal `attribute_scope = 'product'`
el slot `ecom_channel_attributes` que crea es `target_strategy = 'dynamic_field'`,
`target_field = 'website.sale.product.info'`, `category_id = NULL`. El resto de la cadena
(`ecom_attributes` por `code`, `ecom_channel_attribute_map` `custom_attribute` a nivel canal,
`ecom_product_attributes` el valor) es igual para ambos canales.

- Ruta nueva **`POST /api/channel-attribute-values`** `{sku, connectionId?, externalKey, dataType, value}`.
  La vieja `POST /api/marketplaces/mercadolibre/product-attributes` queda como alias (mismo handler).
- El checklist (`GET .../attributes/checklist`) devuelve `attributeScope`. El front
  (`AttributesChecklistCard` → `AddCustomAttributeForm`) muestra un alta libre
  (clave + tipo text/number/boolean/date + valor) **solo** cuando `attributeScope === 'product'`.
  Al guardar, el atributo aparece como fila del checklist (editable inline como el resto);
  borrar su valor lo vuelve a sacar. En modo `product` el checklist lista **solo** los
  atributos con valor de ese producto, no la lista completa de slots del canal.

## Cambios en código

- `infrastructure/mysql/eco-architecture.sql` + `2026-09-09_add_attribute_scope_to_channels.sql`.
- `mysql/channel_repository.go`: `ChannelDTO.AttributeScope` + SELECTs + scan (solo lectura).
- `marketplace/odoo/handler_product_info.go` (nuevo).
- `application/sync/sync_odoo_products.go`: deps nuevas (`channelRepository`,
  `channelAttributesRepository`, `channelAttributeMapRepository`, `productAttributesRepository`,
  `attributeOptionsRepository`), `resolveProductInfoEntries`, `syncProductInfo`, llamada en
  `create`/`update`, return real en `Publish`.
- `application/product_attributes_checklist/service.go`: dep `channelRepository`; rama
  `attribute_scope = 'product'` que salta los cortes por categoría y filtra los slots a los
  que el producto tiene con valor; `ChecklistDTO.AttributeScope`.
- `application/channel_attribute_values/service.go` + handler + `router.go`: `SetValue`
  canal-agnóstico (`ConnectionID` opcional) + ruta `POST /api/channel-attribute-values`.
- `admin-dashboard`: `attributeChecklistSchema.attributeScope`, `useAddChannelAttributeValue`,
  `AddCustomAttributeForm` en la sección Atributos.
- `cmd/main.go`: wiring (Odoo sync, checklist, channel_attribute_values).

## Consecuencias

- En conexiones Odoo el checklist muestra solo los atributos con valor de ese producto, más
  un formulario de alta libre. La lista no crece con lo que se agregó en otros productos.
- Publicar/refrescar un producto Odoo con `is_required` sin valor ahora falla en vez de crear
  un listing incompleto silenciosamente.
- `AllowedSystemFields` sigue sin resolver para el push a Odoo (fuera de alcance; esos datos ya
  viajan en `product.template`).
- `nissanBrandID = 1` y demás hardcodes siguen igual (otro ADR).

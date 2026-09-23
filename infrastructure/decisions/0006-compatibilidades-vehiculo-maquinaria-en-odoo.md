# ADR 0006 — Compatibilidades vehículo/maquinaria en Odoo: catálogo único y sincronización desde core-orchestrator

- **Estado:** aceptado — implementación COMPLETA en código (2026-09-23); pendiente actualizar el módulo en Odoo (ver "Puesta en marcha") y validarlo contra una instancia Odoo 19 (el módulo no se ejecutó en esta sesión)
- **Fecha:** 2026-09-23
- **Servicio:** `services/core-orchestrator`, `odoo-modules/website_sale_machine_catalog`
- **Origen:** el módulo `website_sale_machine_catalog` solo modelaba maquinaria (`machine.brand` →
  `machine.type` → `machine.model`, vinculados a `product.template` por tres many2many) y se
  cargaba a mano en Odoo. core-orchestrator ya tiene dos taxonomías de compatibilidad:
  `ecom_equipment_fitment` (maquinaria: marca, tipo, modelo o serie) y `ecom_vehicle_fitments`
  (vehículos: marca, modelo, rango de años). Los vehículos no tenían dónde vivir en Odoo y
  ninguna compatibilidad se sincronizaba.

## Decisión

### 1. Un solo catálogo en Odoo: el vehículo es un tipo más

Se reutiliza `marca → tipo → modelo` (y con él filtros, rutas `/shop/brand`, "garage" y snippet
de búsqueda) en lugar de crear modelos paralelos para vehículos. Los nombres técnicos `machine.*`
no se renombran (la tienda está en producción); solo cambian etiquetas visibles.

- `machine.type.is_vehicle` (Boolean): los modelos de ese tipo son vehículos. core-orchestrator no
  tiene tabla de tipos de vehículo, así que todo vehículo se archiva bajo un único tipo fijo
  `"Vehículo"` (`odooVehicleMachineTypeName`, `is_vehicle = true`).
- `machine.model.year_start` / `year_end` (Integer; `0` = sin dato): un `machine.model` refleja
  **una fila de `ecom_vehicle_fitments`** (marca + modelo + rango). `year_end = 0` en un vehículo
  significa "desde `year_start` en adelante" (`year_end` NULL en MySQL). Maquinaria deja ambos en 0.
  La unicidad pasa a `(marca, tipo, nombre, año_inicio, año_fin)` (constraint Python; el SQL
  anterior sobre `(marca, tipo, nombre)` se elimina en la migración 19.0.1.1.0).
- `machine.model` con el mismo nombre y varios rangos de años es un solo "modelo" para el cliente:
  la tienda lo lista una vez y ofrece los años que cubre la unión de sus rangos.

Textos visibles: como el tipo "Vehículo" convive con tipos de maquinaria, las etiquetas de la tienda
dejan de decir "maquinaria" cuando hablan del filtro o del catálogo en general ("Tipos", "Modelos",
"Mi garage", "Agregar a Mi garage"); "maquinaria y vehículos" solo aparece donde se nombran ambos
(snippet, ficha de producto, garage vacío). Los menús del backend pasan a "Machinery & Vehicles".

### 2. El año es obligatorio para vehículos

- **Snippet de búsqueda:** al elegir un modelo de vehículo aparece el selector de año, `required`
  (`vehicle_year`).
- **Tienda (`/shop`):** con un modelo de vehículo, el filtro exige `vehicle_year`: coincide con los
  productos ligados a alguna fila del mismo marca/tipo/nombre cuyo rango cubre ese año
  (`machine.model._fitment_ids_for_year`). Sin año válido el modelo **no filtra** (selección
  incompleta), se muestra el aviso "Selecciona el año" y el select de año (`year_required`);
  tampoco se ofrece "Agregar a Mi garage".
- **Garage:** `machine.garage.vehicle_year` (obligatorio si el tipo es vehículo); la unicidad
  incluye el año.
- Maquinaria no cambia: sin años, sin selector.

### 2b. Snippet de búsqueda: cascarón estático + widget que arma el formulario

Odoo guarda un snippet como HTML estático en cada página donde se coloca, así que un
`t-foreach` sobre marcas/tipos dentro de la plantilla congela el catálogo del momento del
arrastre — y con la sincronización que crea marcas y tipos solos, dejaría de ser cierto. Por eso
la plantilla `machine_catalog_search_snippet` es solo un cascarón (`<form>` con
marcadores deshabilitados para el editor) y `machine_search_snippet.js` construye todo en tiempo
de ejecución, también en snippets ya colocados (reemplaza el contenido del `<form>`):

- **Vehículo:** Marca → Modelo → Año (obligatorio, `required` en cuanto hay modelo); el tipo
  "Vehículo" va en un campo oculto.
- **Maquinaria:** Tipo → Marca → Modelo.
- Con `data-search-mode="both"` un selector Vehículo | Maquinaria alterna los dos flujos; si solo
  uno tiene datos, no se muestra. `vehicle` / `machinery` fijan el modo (opción "Search for" en el
  editor web).
- Un endpoint por paso, sin cargar el árbol completo: `search_types` (solo tipos con modelos
  publicados), `search_brands` (marcas con modelos publicados del tipo) y `models` (modelos con
  sus años). El filtrado de `/shop` no cambia.

### 2c. Filtros de `/shop` como checkboxes

Los filtros de la tienda ya no son listas de enlaces en acordeones más chips de "filtro activo":
son cuatro grupos de checkboxes — **Tipo, Marca, Modelo, Año** (el año solo con un modelo de
vehículo) — con la misma apariencia que los filtros nativos de Odoo y ubicados **después** de
ellos (categorías, atributos, precio…): las plantillas se insertan en `products_attributes_filters`
(escritorio) y en `product_attribute_filters_form` (offcanvas móvil) con `priority="20"`, mayor
que la estándar (16). Cada filtro es de elección única: marcar una opción la aplica y
desmarcar la activa la quita. Cada checkbox lleva en `data-url` la URL a la que ir
(`_get_machine_filters`; la de la opción activa es la que limpia ese nivel) y
`machine_filter_checkbox.js` navega a ella al cambiar. Elegir un valor limpia los de abajo (sus
opciones dependen de él); Modelo muestra "Selecciona tipo y marca" hasta tener ambos. Las URLs se
arman siempre sobre `/shop?...` conservando búsqueda, precio y atributos, sin depender del modo de
ruta (`/shop/brand/...`, `/shop/type/...`). Al final del bloque se mantienen "Limpiar todos los
filtros" y "Agregar a Mi garage"; los chips superiores desaparecen.

### 3. `motor`, `position` y `side` no se modelan en Odoo

Son calificadores de la relación producto↔fitment que exige MercadoLibre
(`ecom_product_vehicle_compatibility`), no del vehículo. Odoo no filtra por ellos; el export
deduplica por fitment. Si más adelante se quieren mostrar en la ficha, la vía es un modelo de enlace
`machine.compatibility` (y el many2many pasaría a derivado); no se hace ahora.

### 4. Sincronización desde core-orchestrator

Toda carga de compatibilidades pasa a hacerse en core-orchestrator. `OdooProductSyncService`
llama, después de `syncProductInfo` (en `create` y en el `update` completo/resincronización), a
`syncProductFitments`, que envía la lista **completa** de compatibilidades del producto (incluida
vacía) a `product.template.ecom_sync_fitments(fitments)` por Odoo JSON-2:

```json
{ "ids": [<product.template.id>],
  "fitments": [{
    "ecom_ref": "vehicle_fitment:12" | "equipment_fitment:7",
    "name": "Versa", "year_start": 2012, "year_end": 2019,
    "brand": { "ecom_ref": "brand:1", "name": "Nissan" },
    "type":  { "ecom_ref": "equipment_type:3" (solo maquinaria), "name": "Vehículo", "is_vehicle": true }
  }] }
```

- `ecom_ref` (Char, único, NULL para lo cargado a mano) en marca, tipo y modelo identifica cada
  registro en core-orchestrator (`brand:<ecom_brands.id>`, `equipment_type:<id>`,
  `vehicle_fitment:<id>`, `equipment_fitment:<id>`); el tipo "Vehículo" no lleva `ecom_ref` y se
  reconoce por nombre.
- **Adopción del catálogo manual:** si no hay coincidencia por `ecom_ref`, se adopta el registro
  cargado a mano con el mismo nombre (marca/tipo) o mismos marca/tipo/nombre/años (modelo) y se le
  estampa el `ecom_ref`; no se duplica lo existente.
- **Alcance de la sincronización:** el método reemplaza solo los modelos *gestionados* del producto
  (con `ecom_ref`); los cargados a mano nunca se quitan. `brand_ids` y `machine_type_ids` se
  mantienen coherentes con los modelos: se quitan las marcas/tipos que solo respaldaba un modelo
  removido y se agregan los de los modelos finales; una marca/tipo asignado a mano sin modelo no se
  toca. Si el conjunto no cambia, no escribe nada.
- Equipos: `machine.model.name` = `model` + `serie` (uno de los dos puede faltar).
- Un fallo se reporta como en `website.sale.product.info`: el listing ya existe y el siguiente
  `update` completo lo reintenta.

Origen de los datos: `ProductFitmentsExportRepository` (solo lectura; vehículos con `DISTINCT` por
fitment, ignorando motor/position/side).

## Consecuencias

- Agregar o quitar una compatibilidad en core-orchestrator **no** dispara por sí sola una llamada a
  Odoo: se refleja en el siguiente `update` completo del listing (resincronización).
- Un producto sin compatibilidades genera igualmente una llamada por sync (barata; necesaria para
  propagar el borrado de la última).
- Orden de despliegue: **actualizar el módulo en Odoo a 1.1.0 antes** de desplegar este cambio de
  core-orchestrator; sin `ecom_sync_fitments` cada publicación/update de Odoo falla en ese paso.
- Marcas y modelos con el mismo nombre en `ecom_brands` y en catálogo manual (mayúsculas
  distintas) se adoptan con `=ilike`; nombres divergentes crean registros nuevos.

## Puesta en marcha

1. Copiar `odoo-modules/website_sale_machine_catalog` al addons path y actualizar el módulo
   (`-u website_sale_machine_catalog`): corre `migrations/19.0.1.1.0/pre-migration.py`, agrega
   columnas y recalcula.
2. Ejecutar una resincronización completa de los listings de Odoo para poblar/adoptar el catálogo.
3. Revisar en Odoo que ningún producto quedó con marca asignada a mano sin modelo que se quiera
   conservar (no se toca, pero conviene confirmarlo tras el primer sync).

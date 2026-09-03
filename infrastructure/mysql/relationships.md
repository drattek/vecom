# Relaciones principales de la base de datos

## Mapa de relaciones por dominio

### Catálogo y productos
- `ecom_products.brand_id` → `ecom_brands.id`
- `ecom_products.category_id` → `ecom_categories.id`
- `ecom_products.source_id` → `ecom_sources.id`
- `ecom_products.id` → `ecom_product_part_numbers.product_id`
- `ecom_products.id` → `ecom_product_dimensions.product_id`
- `ecom_products.id` → `ecom_product_seo.product_id`
- `ecom_products.id` → `ecom_product_media.product_id`
- `ecom_products.id` → `ecom_product_images.product_id`
- `ecom_products.id` → `ecom_product_videos.product_id`
- `ecom_product_media.file_id` → `ecom_files.id`
- `ecom_product_images.file_id` → `ecom_files.id`
- `ecom_product_videos.file_id` → `ecom_files.id`

### Almacenamiento y archivos
- `ecom_files.disk_id` → `ecom_storage_disks.id`
- `ecom_channels.icon_id` → `ecom_files.id`

### Inventario y logística
- `ecom_warehouses.branch_id` → `ecom_branches.id`
- `ecom_product_stock.product_id` → `ecom_products.id`
- `ecom_product_stock.branch_id` → `ecom_branches.id`
- `ecom_product_stock.warehouse_id` → `ecom_warehouses.id`
- `ecom_stock_movements.product_id` → `ecom_products.id`
- `ecom_stock_movements.branch_id` → `ecom_branches.id`
- `ecom_stock_movements.warehouse_id` → `ecom_warehouses.id`

### Precios y monedas
- `ecom_exchange_rates.from_currency_id` → `ecom_currencies.id`
- `ecom_exchange_rates.to_currency_id` → `ecom_currencies.id`
- `ecom_price_list.currency` → `ecom_currencies.id`
- `ecom_product_prices.product_id` → `ecom_products.id`
- `ecom_product_prices.price_list_id` → `ecom_price_list.id`
- `ecom_product_prices.currency` → `ecom_currencies.id`
- `ecom_price_history.product_id` → `ecom_products.id`
- `ecom_price_history.price_list_id` → `ecom_price_list.id`
- `ecom_price_history.currency_id` → `ecom_currencies.id`
- `ecom_pricing_formulas.brand_id` → `ecom_brands.id` (nullable: null = comodín "cualquier marca")
- `ecom_pricing_formulas.connection_id` → `ecom_channel_connections.id` (nullable: null = comodín "cualquier conexión")
- `ecom_pricing_formulas.price_list_id` → `ecom_price_list.id` (nullable: null = comodín "cualquier lista de precios"; es la lista que ganó al resolver el precio efectivo del producto, no una lista elegida a mano). Los tres comodines a la vez es el default universal. `PricingFormulaRepository.Resolve` elige, para (brand_id, connection_id, price_list_id) de un producto/conexión/lista dados, la fila más específica: prioridad de eje `connection_id` > `brand_id` > `price_list_id`, con más dimensiones coincidentes ganando ante menos. Aplica al calcular el precio publicado en MercadoLibre y Odoo a partir del precio base efectivo.

### Compatibilidad de vehículos y equipos
- `ecom_vehicle_fitments.brand_id` → `ecom_brands.id`
- `ecom_product_vehicle_compatibility.product_id` → `ecom_products.id`
- `ecom_product_vehicle_compatibility.vehicle_fitment_id` → `ecom_vehicle_fitments.id`
- `ecom_equipment_fitment.brand_id` → `ecom_brands.id`
- `ecom_equipment_fitment.equipment_type` → `ecom_equipment_types.id`
- `ecom_product_equipment_compatibility.product_id` → `ecom_products.id`
- `ecom_product_equipment_compatibility.equipment_fitment_id` → `ecom_equipment_fitment.id`
- `ecom_pending_product_vehicle_fitments.vehicle_fitment_id` → `ecom_vehicle_fitments.id`
- `ecom_pending_product_vehicle_fitments.resolved_product_id` → `ecom_products.id` (nulo hasta que se resuelve; no hay FK sobre `sku`, ya que puede no existir todavía en `ecom_products`)

### Sucesión de números de parte
- `ecom_part_number_supersessions.source_id` → `ecom_sources.id`
- `ecom_part_number_supersessions.old_product_id` → `ecom_products.id` (nulo hasta que se resuelve; no hay FK sobre `old_part_number`, ya que la pieza reemplazada puede no existir todavía en `ecom_products`)
- `ecom_part_number_supersessions.new_product_id` → `ecom_products.id` (nulo hasta que se resuelve; no hay FK sobre `new_part_number`, por la misma razón)

### Canales y conexiones
- `ecom_channels.id` → `ecom_channel_connections.channel_id`
- `ecom_channel_connections.currency_id` → `ecom_currencies.id`
- `ecom_channel_connections.id` → `ecom_connection_credentials.connection_id`
- `ecom_channel_connections.id` → `ecom_connection_settings.connection_id`
- `ecom_channel_connections.id` → `ecom_connection_status.connection_id`
- `ecom_channel_parameters.channel_id` → `ecom_channels.id`

## Reglas de uso de relaciones
- Cuando se necesite consultar datos de un producto completo, recorrer primero las tablas de dominio del producto y luego las tablas de soporte relacionadas.
- Para joins de negocio, priorizar relaciones explícitas de llave foránea sobre usar subconsultas redundantes.
- Para operaciones de stock, no mezclar `ecom_product_stock` con `ecom_stock_movements` sin tener claro si se busca el estado actual o el historial.
- Para operaciones de pricing, considerar que `ecom_product_prices` es la tabla de estado actual y `ecom_price_history` es la tabla de trazabilidad.
- Para conexiones externas, `ecom_channel_connections` es la entidad principal; las credenciales y ajustes deben manejarse como datos dependientes de esa conexión.
- Para compatibilidad de vehículos, `ecom_pending_product_vehicle_fitments` no es una relación de producto activa: es una cola de resolución. Un SKU ahí no implica que el producto exista; solo se vuelve compatibilidad real (`ecom_product_vehicle_compatibility`) cuando se crea el producto con ese SKU.

## Recomendaciones de navegación
- Si el objetivo es entender un producto, seguir el recorrido: `ecom_products` → marca/categoría → stock → precios → media.
- Si el objetivo es entender una integración de canal, seguir: `ecom_channels` → `ecom_channel_connections` → `ecom_connection_credentials` / `ecom_connection_settings` / `ecom_connection_status`.
- Si el objetivo es entender un archivo, seguir: `ecom_files` → `ecom_storage_disks` y luego las tablas que referencian ese `file_id`.
- Si el objetivo es entender compatibilidad de vehículos, seguir: `ecom_vehicle_fitments` → `ecom_product_vehicle_compatibility` para lo ya resuelto, y `ecom_pending_product_vehicle_fitments` para lo pendiente de un SKU que aún no tiene producto.

## Consideraciones de integridad
- No se recomienda crear registros en tablas dependientes sin validar primero la existencia del padre.
- En cambios que afecten el catálogo completo, revisar si hay dependencias en stock, precios, media o canales para evitar inconsistencias.
- Cuando una tabla tenga `deleted_at`, la relación debe evaluarse como activa solo si el padre e hijo también están activos.

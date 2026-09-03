# Instrucciones de base de datos para el proyecto

## Contexto general
- La base de datos está orientada a un modelo comercial y operativo para productos, inventario, precios, medios, canales y conexiones externas.
- El esquema usa el prefijo `ecom_` para todas las tablas, incluidas las de dominio y las de acceso API/autenticación (`ecom_api_user`, `ecom_api_token`).
- La mayoría de las tablas de negocio incluyen columnas de auditoría: `created_by`, `updated_by`, `created_at`, `updated_at` y, cuando aplica, `deleted_at`.
- El modelo asume que los registros eliminados no deben borrarse físicamente; deben marcarse con `deleted_at` y filtrarse en lecturas normales.

## Reglas de decisión para interactuar con el proyecto
1. Respetar integridad referencial y no crear registros huérfanos.
2. Preferir extender tablas existentes antes que crear nuevas tablas paralelas para un mismo concepto.
3. Al crear datos de negocio, siempre completar `created_by` y valores por defecto compatibles con los enums.
4. Al actualizar datos de negocio, actualizar `updated_by` y conservar `updated_at`.
5. Cuando una tabla tenga `deleted_at`, asumir que los registros activos son aquellos con `deleted_at IS NULL`.
6. Para cambios sensibles como credenciales, configuraciones o estados de conexión, no asumir que el valor debe guardarse directamente en texto plano; usar la capa de aplicación o el flujo previsto por el dominio.
7. Mantener consistencia entre tablas de soporte: si un producto tiene marca, categoría, stock, precios y medios, preferir leerlos de forma conjunta en vez de trabajar con datos aislados.

## Convenciones del esquema
- `ecom_api_user` es la fuente de identidad para operaciones de auditoría; no debe usarse como tabla de negocio directa.
- `ecom_products` es la tabla central del catálogo y sirve como punto de unión para dimensiones, SEO, media, stock y precios.
- `ecom_sources` identifica el origen de un producto (código y nombre); `ecom_products.source_id` es obligatorio y referencia esta tabla.
- `ecom_brands` y `ecom_categories` son dimensiones principales del catálogo y deben mantenerse relativamente estables.
- `ecom_branches` y `ecom_warehouses` representan la estructura logística; el stock real vive en `ecom_product_stock`.
- `ecom_channels` y `ecom_channel_connections` representan integraciones externas; las credenciales y configuraciones deben tratarse como datos sensibles.
- `ecom_vehicle_fitments` y `ecom_equipment_fitment` son las dimensiones de compatibilidad (vehículo/equipo); `ecom_product_vehicle_compatibility` y `ecom_product_equipment_compatibility` son las tablas de enlace hacia `ecom_products`.
- `ecom_pending_product_vehicle_fitments` es una tabla de staging: guarda compatibilidades de vehículo importadas (p. ej. vía bulk import) cuyo SKU todavía no existe en `ecom_products`. No tiene FK sobre `sku` (a propósito, porque el producto puede no existir aún). Cuando se crea un producto con ese SKU, se resuelve automáticamente hacia `ecom_product_vehicle_compatibility` y la fila pendiente se conserva (no se borra), marcada con `resolved_at`/`resolved_product_id` para trazabilidad.
- `ecom_part_number_supersessions` guarda el estado *actual* de la sucesión de número de parte por pieza (a lo más una fila por `source_id`+`old_part_number`, igual que `ecom_product_stock`/`ecom_product_prices` — no es un log de eventos). Hoy la puebla la columna `PROD_SUPERSESION` de la vista Nissan (`RE_VEXISTENCIAS`), pero la tabla está diseñada de forma source-agnostic (columna `source_id`) por si otro origen empieza a reportar sucesiones más adelante. Representa una pieza descontinuada reemplazada por otra con número de parte distinto (misma pieza física). Es una tabla de staging igual que `ecom_pending_product_vehicle_fitments`, pero con ambos lados potencialmente pendientes: ni `old_part_number` (la pieza descontinuada) ni `new_part_number` (la que la reemplaza) tienen garantizado tener ya un registro en `ecom_products`, así que `old_product_id`/`new_product_id` son nullable y se resuelven de forma independiente cuando el sync de Nissan crea/actualiza el producto correspondiente (`nissanSyncTx.run` en `sync_nissan.go`). `ecom_products` tiene `unique key (source_id, part_number)` precisamente para que esa resolución no sea ambigua. Si `PROD_SUPERSESION` cambia de valor para la misma pieza, se actualiza la fila existente en vez de insertar una nueva, y eso limpia `new_product_id`/`new_resolved_at` porque el vínculo resuelto anteriormente ya no aplica al nuevo valor.

## Recomendaciones prácticas
- Antes de crear un nuevo registro, verificar si ya existe una tabla o relación equivalente para el concepto.
- Para consultas de catálogo, usar el enfoque de lectura jerárquica: producto → marca/categoría → stock → precios → media.
- Para operaciones de inventario, revisar primero `ecom_product_stock` y luego `ecom_stock_movements` para tener contexto histórico.
- Para operaciones de pricing, consultar `ecom_price_list`, `ecom_product_prices` y `ecom_price_history` antes de modificar precios.
- Para integraciones de marketplace o canales, revisar `ecom_channel_connections`, `ecom_connection_credentials`, `ecom_connection_settings` y `ecom_connection_status`.
- Para compatibilidad de vehículos, si un SKU no se encuentra en `ecom_products` al importar, no se debe forzar la creación del producto ni descartar el dato: debe quedar en `ecom_pending_product_vehicle_fitments` hasta que el producto exista.

## Criterios para tomar decisiones
- Si el cambio afecta negocio y no solo infraestructura, probablemente debe pasar por las tablas de dominio (`products`, `stock`, `prices`, `channels`) y no solo por tablas auxiliares.
- Si se necesita trazabilidad, priorizar el uso de `created_by`/`updated_by` y conservar la relación con el usuario correspondiente.
- Si se detecta ambigüedad entre tablas de soporte y tablas principales, tomar como base la tabla central del módulo y luego extender con sus tablas auxiliares.

## Acceso desde `core-orchestrator` (Go)
- Pool único (`mysql.NewConnection`) con límites `MYSQL_MAX_OPEN_CONNS` / `MYSQL_MAX_IDLE_CONNS` / `MYSQL_CONN_MAX_LIFETIME_MINUTES` / `MYSQL_CONN_MAX_IDLE_TIME_MINUTES`.
- Cada repo (`internal/infrastructure/mysql/*`) se construye con `mysql.Querier`, así puede correr contra el pool o contra una `*sql.Tx`.
- Operación que escribe en **varias tablas** (p. ej. actualizar el estado actual + insertar en la tabla de histórico como `ecom_product_stock`/`ecom_stock_movements` o `ecom_product_prices`/`ecom_price_history`, o resolver marca + asignarla a un producto): correr dentro de `mysql.WithinTx` para que sea todo-o-nada.
- Operación de **una sola sentencia**: repo sobre el pool, sin transacción.
- Detalle y estado de la migración: `infrastructure/decisions/0001-persistencia-transacciones-y-context.md`.

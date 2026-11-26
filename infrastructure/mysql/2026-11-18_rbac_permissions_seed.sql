-- Catálogo inicial de permisos (resource.action) del admin-dashboard.
-- Idempotente: se puede re-ejecutar para agregar permisos nuevos o corregir
-- descripciones (UNIQUE (resource, action) + ON DUPLICATE KEY UPDATE).
--
-- Fuera del catálogo a propósito (solo admin, sin permiso asignable): rutas
-- sin vista en el dashboard — /api/sources, /api/attributes*,
-- /api/channel-attributes*, /api/channel-parameters*, /api/migration/* y los
-- endpoints temp-* / debug de MercadoLibre.

-- ---------------------------------------------------------------------------
-- 1) Módulos con CRUD estándar: view / create / edit / delete
-- ---------------------------------------------------------------------------
-- El SELECT va envuelto en una tabla derivada (src): con un JOIN directo,
-- MySQL interpreta el "ON" de ON DUPLICATE KEY UPDATE como la condición del
-- JOIN y marca DUPLICATE como inválido.
INSERT INTO ecom_permissions (resource, action, description)
SELECT src.resource, src.action, src.description
FROM (
SELECT r.resource, a.action, CONCAT(a.label, ' ', r.label) AS description
FROM (
    SELECT 'storage_disks'        AS resource, 'discos de almacenamiento'        AS label UNION ALL
    SELECT 'storage_files',                    'archivos almacenados'                     UNION ALL
    SELECT 'branches',                         'sucursales'                               UNION ALL
    SELECT 'warehouses',                       'almacenes'                                UNION ALL
    SELECT 'currencies',                       'divisas'                                  UNION ALL
    SELECT 'exchange_rates',                   'tipos de cambio'                          UNION ALL
    SELECT 'price_lists',                      'listas de precios'                        UNION ALL
    SELECT 'pricing_formulas',                 'fórmulas de precios'                      UNION ALL
    SELECT 'channels',                         'canales de marketplace'                   UNION ALL
    SELECT 'connections',                      'conexiones de canal'                      UNION ALL
    SELECT 'brands',                           'marcas'                                   UNION ALL
    SELECT 'categories',                       'categorías'                               UNION ALL
    SELECT 'vehicle_fitments',                 'compatibilidades de vehículo (catálogo)'  UNION ALL
    SELECT 'equipment_fitments',               'compatibilidades de equipo (catálogo)'    UNION ALL
    SELECT 'equipment_types',                  'tipos de equipo'                          UNION ALL
    SELECT 'roles',                            'roles y permisos'                         UNION ALL
    SELECT 'products',                         'productos'                                UNION ALL
    SELECT 'product_media',                    'multimedia del producto'                  UNION ALL
    SELECT 'product_part_numbers',             'números de parte del producto'
) r
CROSS JOIN (
    SELECT 'view'   AS action, 'Ver'      AS label UNION ALL
    SELECT 'create',           'Crear'             UNION ALL
    SELECT 'edit',             'Editar'            UNION ALL
    SELECT 'delete',           'Eliminar'
) a
) src
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- ---------------------------------------------------------------------------
-- 2) Permisos específicos (vistas de solo lectura y acciones especiales)
-- ---------------------------------------------------------------------------
INSERT INTO ecom_permissions (resource, action, description) VALUES
    -- Navegación general
    ('dashboard',            'view',              'Ver el dashboard'),
    ('reports',              'view',              'Ver reportes de publicaciones'),
    ('meli_listings',        'view',              'Ver la vista de publicaciones de MercadoLibre'),

    -- Usuarios (sin delete: no existe baja de usuarios hoy)
    ('users',                'view',              'Ver usuarios'),
    ('users',                'create',            'Crear usuarios'),
    ('users',                'edit',              'Editar usuarios, su rol y permisos extra'),

    -- Productos: vistas y herramientas masivas
    ('products_pending',     'view',              'Ver productos pendientes'),
    ('products_channels',    'view',              'Ver productos por canal y conexión'),
    ('products_images',      'view',              'Ver la herramienta de imágenes de productos'),
    ('products_images',      'import',            'Importar imágenes de productos por SKU'),
    ('products_import',      'view',              'Ver la importación masiva de publicaciones de MercadoLibre'),
    ('products_import',      'import',            'Subir Excel de importación masiva de MercadoLibre'),
    ('products_import',      'retry',             'Reintentar elementos de la importación masiva de MercadoLibre'),
    ('products_import',      'export',            'Exportar lotes de la importación masiva de MercadoLibre'),
    ('products_bulk_update', 'view',              'Ver la actualización masiva de productos'),
    ('products_bulk_update', 'import',            'Subir Excel de actualización masiva de productos'),
    ('products_bulk_update', 'retry',             'Reintentar elementos de la actualización masiva'),
    ('products_bulk_update', 'export',            'Exportar lotes de la actualización masiva'),

    -- Detalle de producto (una pestaña = un recurso)
    ('product_pricing',         'view',           'Ver precios del producto'),
    ('product_inventory',       'view',           'Ver inventario del producto'),
    ('product_inventory',       'create',         'Registrar movimientos de inventario'),
    ('product_attributes',      'view',           'Ver atributos del producto'),
    ('product_attributes',      'edit',           'Editar atributos del producto (dimensiones, SEO, atributos de canal)'),
    ('product_compatibilities', 'view',           'Ver compatibilidades del producto'),
    ('product_compatibilities', 'create',         'Agregar compatibilidades al producto'),
    ('product_compatibilities', 'delete',         'Quitar compatibilidades del producto'),
    ('product_sync',            'view',           'Ver la sincronización del producto por conexión'),
    ('product_sync',            'publish',        'Publicar / importar publicación / elegir categoría desde la sincronización del producto'),

    -- Publicaciones y operaciones en marketplaces
    ('channel_listings',     'view',              'Ver publicaciones, historial y cola de sincronización'),
    ('channel_listings',     'publish',           'Publicar productos en un canal'),
    ('channel_listings',     'resync',            'Resincronizar / refrescar publicaciones'),
    ('channel_listings',     'close',             'Cerrar publicaciones'),
    ('mercadolibre',         'sync',              'Ejecutar sincronizaciones de MercadoLibre (precio/stock, estado, compatibilidades)'),

    -- Acciones especiales de módulos CRUD
    ('price_lists',          'import',            'Importar precios masivamente a una lista de precios'),
    ('connections',          'manage_credentials','Ver y editar credenciales y configuración de conexiones'),
    ('brands',               'bulk_assign',       'Asignar marcas masivamente por SKU'),
    ('categories',           'import',            'Importar categorías desde un canal y mapearlas'),
    ('vehicle_fitments',     'import',            'Importar compatibilidades de vehículo masivamente'),

    -- Ventas (ADR 0010)
    ('sales',                'view',              'Ver las ventas y su detalle, incluidos los datos del cliente'),
    ('sales',                'sync',              'Ejecutar "Sincronizar ahora" en la sección de ventas')
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- Verificación rápida:
--   SELECT resource, GROUP_CONCAT(action ORDER BY action) AS actions
--   FROM ecom_permissions GROUP BY resource ORDER BY resource;

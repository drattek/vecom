-- Publicar artículos con foto, precio y datos completos pero stock=0 como
-- "suspendidos/pausados" en vez de omitirlos. Se decide por conexión.
--
-- Dos columnas:
--
-- 1. ecom_channel_connections.sync_zero_stock_as_paused — flag por conexión.
--    Cuando está en 1, las publicaciones (Create/Refresh) tratan qty=0 como
--    señal para marcar paused (MELI) o sale_ok=false (Odoo) en vez de
--    enviar la cantidad real y dejar el listing comprable.
--
-- 2. ecom_channel_product_map.paused_reason — por qué está en 'paused' hoy.
--    'zero_stock' lo pone el refresher automáticamente cuando el toggle está
--    activo y qty cae a 0; esos listings SÍ se reactivan solos cuando vuelve
--    el stock. 'user' lo pone el admin desde el dashboard; esos listings
--    NUNCA se reactivan por sync. NULL cuando status no es 'paused'.
--
-- Ejecutar a mano contra la base de core-orchestrator.

ALTER TABLE ecom_channel_connections
    ADD COLUMN sync_zero_stock_as_paused TINYINT(1) NOT NULL DEFAULT 0
    AFTER allows_multiple_listings;

ALTER TABLE ecom_channel_product_map
    ADD COLUMN paused_reason ENUM('user','zero_stock') DEFAULT NULL
    AFTER status;

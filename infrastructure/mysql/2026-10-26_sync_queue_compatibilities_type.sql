-- Botón "Sincronizar compatibilidades" de una conexión (Odoo): encola un
-- producto por publicación existente para que el MarketplaceWorker empuje solo
-- sus compatibilidades de maquinaria/vehículos (ADR 0006), sin tocar precio,
-- stock, atributos ni imágenes.
--
-- Las filas usan un valor nuevo de ecom_channel_sync_queue.sync_type. La unique
-- key (product_id, connection_id, sync_type, active_row) del ADR 0003 ya
-- incluye sync_type, así que conviven con las filas 'listing' del mismo par.
--
-- Ejecutar a mano contra la base de core-orchestrator ANTES de desplegar.

ALTER TABLE ecom_channel_sync_queue
    MODIFY COLUMN sync_type ENUM('price', 'stock', 'full', 'listing', 'compatibilities') NOT NULL DEFAULT 'full';

-- Botón "Resincronizar" global de una conexión (MercadoLibre y Odoo): encola un
-- producto por publicación existente para que el MarketplaceWorker ejecute el
-- Resync completo (precio, stock y demás campos mutables, sin filtrar por cambios).
--
-- Usa un valor nuevo de ecom_channel_sync_queue.sync_type; la unique key del
-- ADR 0003 ya incluye sync_type, así que convive con las filas 'listing' y
-- 'compatibilities' del mismo par.
--
-- Ejecutar a mano contra la base de core-orchestrator ANTES de desplegar.

ALTER TABLE ecom_channel_sync_queue
    MODIFY COLUMN sync_type ENUM('price', 'stock', 'full', 'listing', 'compatibilities', 'resync') NOT NULL DEFAULT 'full';

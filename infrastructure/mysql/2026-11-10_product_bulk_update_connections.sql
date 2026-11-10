-- Actualización masiva de productos: conexiones a resincronizar. Al subir el
-- Excel se pueden marcar conexiones de canal (MercadoLibre, Odoo...); después de
-- guardar cada fila en la base local, el worker resincroniza el producto en las
-- publicaciones que ya tenga en esas conexiones (mismo flujo que el botón
-- "Resincronizar" del detalle del producto). NULL = no resincronizar nada.
--
-- Ejecutar a mano contra la base de core-orchestrator, DESPUÉS de
-- 2026-11-10_product_bulk_update.sql.

ALTER TABLE ecom_product_bulk_update_batches
    ADD COLUMN sync_connection_ids JSON DEFAULT NULL AFTER storage_disk_id;

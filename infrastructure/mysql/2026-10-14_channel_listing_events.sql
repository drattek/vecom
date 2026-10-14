-- Bitácora (append-only) de todo lo que le pasa a una publicación por canal:
-- cada intento de publicar (exitoso o fallido), cada refresco/resync que
-- empujó cambios o falló, cada pausa/reactivación por stock y cada cambio de
-- estado que reporta el marketplace. Nunca se actualiza ni se borra: es el
-- histórico para ver todos los reintentos y lo sucedido.
--
-- Antes, un intento fallido sólo vivía en la respuesta HTTP; un producto que
-- nunca se publicó no dejaba rastro. "No publicadas" se deriva de aquí: el
-- último evento action='publish' de (producto, conexión, compatibilidad) con
-- outcome='error' y sin fila viva en ecom_channel_product_map.
--
-- No se guardan como filas de ecom_channel_product_map porque esa tabla la
-- leen refresh/resync/discovery/resumen del canal como publicaciones vivas.
--
-- Ejecutar a mano contra la base de core-orchestrator.

CREATE TABLE ecom_channel_listing_events (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    product_id BIGINT UNSIGNED NOT NULL,
    connection_id BIGINT UNSIGNED NOT NULL,
    vehicle_fitment_id BIGINT UNSIGNED DEFAULT NULL,
    vehicle_fitment_key BIGINT UNSIGNED AS (COALESCE(vehicle_fitment_id, 0)) STORED,
    channel_product_map_id BIGINT UNSIGNED DEFAULT NULL,
    external_id VARCHAR(100) DEFAULT NULL,
    action ENUM('publish','refresh','resync','pause','reactivate','status_sync') NOT NULL,
    outcome ENUM('success','error') NOT NULL,
    status_before VARCHAR(32) DEFAULT NULL,
    status_after VARCHAR(32) DEFAULT NULL,
    message TEXT DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    KEY idx_cle_listing (product_id, connection_id, vehicle_fitment_key, id),
    KEY idx_cle_connection_action (connection_id, action, id),
    KEY idx_cle_created (created_at),
    CONSTRAINT fk_cle_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_cle_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id)
);

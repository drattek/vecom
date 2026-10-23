-- Importación masiva de publicaciones de MercadoLibre por Excel (columnas sku y
-- meli_id). Cada archivo subido es un lote (ecom_meli_import_batches) y cada
-- fila del Excel un ítem (ecom_meli_import_items) que un worker del servidor
-- procesa de uno en uno, con el mismo flujo que el botón "Importar publicación
-- existente" del detalle del producto (meli_listing_import).
--
-- Estados del ítem:
--   pending    en cola, esperando al worker
--   running    el worker lo está importando
--   done       importado y publicación vinculada
--   failed     el import no terminó (número de parte distinto, publicación
--              cerrada, error de MercadoLibre...); se puede reintentar
--   missing    el SKU no existe en ecom_products ("Faltante"); no se importa
--   duplicate  el producto ya tiene una publicación vinculada en esa conexión;
--              se omite
--
-- Ejecutar a mano contra la base de core-orchestrator.

CREATE TABLE ecom_meli_import_batches (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    connection_id BIGINT UNSIGNED NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    total_rows INT UNSIGNED NOT NULL DEFAULT 0,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    KEY idx_mib_connection (connection_id),
    CONSTRAINT fk_mib_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_mib_created FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_mib_updated FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

CREATE TABLE ecom_meli_import_items (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    batch_id BIGINT UNSIGNED NOT NULL,
    connection_id BIGINT UNSIGNED NOT NULL,
    sheet_row INT UNSIGNED NOT NULL,
    sku VARCHAR(100) NOT NULL,
    meli_id_raw VARCHAR(255) NOT NULL DEFAULT '',
    external_id VARCHAR(100) DEFAULT NULL,
    product_id BIGINT UNSIGNED DEFAULT NULL,
    status ENUM('pending','running','done','failed','missing','duplicate') NOT NULL DEFAULT 'pending',
    error_message TEXT DEFAULT NULL,
    listing_title VARCHAR(255) DEFAULT NULL,
    steps JSON DEFAULT NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    started_at TIMESTAMP NULL DEFAULT NULL,
    finished_at TIMESTAMP NULL DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    KEY idx_mii_queue (status, id),
    KEY idx_mii_batch (batch_id, status),
    KEY idx_mii_connection_status (connection_id, status),
    KEY idx_mii_sku (sku),
    KEY idx_mii_external (external_id),
    CONSTRAINT fk_mii_batch FOREIGN KEY (batch_id) REFERENCES ecom_meli_import_batches(id),
    CONSTRAINT fk_mii_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_mii_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_mii_created FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_mii_updated FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

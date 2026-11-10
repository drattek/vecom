-- Actualización masiva de productos por Excel (Productos → botón "Actualización
-- masiva"). Cada archivo subido es un lote (ecom_product_bulk_update_batches) y
-- cada fila del Excel un ítem (ecom_product_bulk_update_items) que un worker del
-- servidor procesa en segundo plano (módulo product_bulk_update), igual que la
-- importación masiva de publicaciones de MercadoLibre (2026-10-23_meli_bulk_import.sql).
--
-- El lote guarda la source y el disco de almacenamiento elegidos al subir el
-- archivo. El ítem guarda lo que dice la fila del Excel en `payload` (JSON) para
-- que el worker y los reintentos no dependan del archivo.
--
-- Estados del ítem:
--   pending    en cola, esperando al worker
--   running    el worker lo está procesando
--   done       todos los pasos terminaron bien (o no había nada que cambiar)
--   failed     la fila es inválida o algún paso falló (marca inexistente, SKU de
--              otra source, URL de imagen rota...); se puede corregir y reintentar
--   missing    el SKU no existe en ecom_products; no se actualiza
--
-- Ejecutar a mano contra la base de core-orchestrator.

CREATE TABLE ecom_product_bulk_update_batches (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    source_id BIGINT UNSIGNED NOT NULL,
    storage_disk_id BIGINT UNSIGNED NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    total_rows INT UNSIGNED NOT NULL DEFAULT 0,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    KEY idx_pbub_source (source_id),
    CONSTRAINT fk_pbub_source FOREIGN KEY (source_id) REFERENCES ecom_sources(id),
    CONSTRAINT fk_pbub_disk FOREIGN KEY (storage_disk_id) REFERENCES ecom_storage_disks(id),
    CONSTRAINT fk_pbub_created FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_pbub_updated FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

CREATE TABLE ecom_product_bulk_update_items (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    batch_id BIGINT UNSIGNED NOT NULL,
    sheet_row INT UNSIGNED NOT NULL,
    sku VARCHAR(255) NOT NULL,
    product_id BIGINT UNSIGNED DEFAULT NULL,
    payload JSON DEFAULT NULL,
    status ENUM('pending','running','done','failed','missing') NOT NULL DEFAULT 'pending',
    error_message TEXT DEFAULT NULL,
    steps JSON DEFAULT NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    started_at TIMESTAMP NULL DEFAULT NULL,
    finished_at TIMESTAMP NULL DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    KEY idx_pbui_queue (status, id),
    KEY idx_pbui_batch (batch_id, status),
    KEY idx_pbui_sku (sku),
    CONSTRAINT fk_pbui_batch FOREIGN KEY (batch_id) REFERENCES ecom_product_bulk_update_batches(id),
    CONSTRAINT fk_pbui_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_pbui_created FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_pbui_updated FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

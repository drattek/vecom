-- ADR 0005 — Selección manual de categoría por conexión antes de publicar.
--
-- Tabla nueva ecom_channel_product_category_selection: registra, por
-- (product_id, connection_id), la categoría externa que el usuario eligió
-- en la pestaña Sincronización del detalle de producto ANTES de que exista
-- ninguna publicación (ecom_channel_product_map) para esa conexión.
-- category_id es la categoría local hoja que category_import.Service.Import
-- resolvió/creó para external_category_id — se guarda aquí solo para no
-- tener que re-derivarla, nunca se escribe en ecom_products.category_id.
--
-- No se puede reusar ecom_channel_product_map para esto: channel_listings.
-- publishOne trata cualquier fila con status distinto de 'closed' como "ya
-- publicado" y saltearía la publicación real si se precreara una fila ahí
-- antes de publicar.
--
-- Ejecutar a mano contra la base de core-orchestrator.

CREATE TABLE ecom_channel_product_category_selection (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    product_id BIGINT UNSIGNED NOT NULL,
    connection_id BIGINT UNSIGNED NOT NULL,
    category_id BIGINT UNSIGNED NOT NULL,
    external_category_id VARCHAR(255) NOT NULL,
    external_category_name VARCHAR(255) DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    UNIQUE KEY uq_product_connection_category_selection (product_id, connection_id),
    CONSTRAINT fk_pcs_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_pcs_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_pcs_category FOREIGN KEY (category_id) REFERENCES ecom_categories(id)
);

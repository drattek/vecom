-- Shadow MELI notifications processor
--
-- La tabla ecom_meli_notifications solo guardaba el "aviso" de MercadoLibre
-- (notification_id, topic, resource) sin resolver el recurso que apunta ni
-- promover el status más allá de 'pending'. Esta migración:
--
-- 1) Agrega las columnas que el procesador shadow necesita en la tabla de
--    notificaciones:
--      - processed_payload: JSON crudo del GET al recurso de MELI (orden,
--        envío, pago, ...) exactamente como lo devolvió MELI al procesarse.
--        Se guarda para auditar y para que el consumidor Odoo (fase 2)
--        pueda leer sin volver a pegarle a MELI.
--      - connection_id ya existe en la tabla; el procesador lo rellena al
--        resolver meli_user_id → conexión.
--
-- 2) Crea ecom_meli_orders y ecom_meli_order_items, las tablas destino del
--    topic orders_v2. En modo shadow el procesador escribe aquí; nada más
--    (ni sale.order en Odoo, ni movimientos de stock). La fase 2 leerá
--    estas tablas para construir el sale.order en Odoo.
--
-- 3) Crea ecom_meli_shipments para el topic shipments. Mismo criterio
--    shadow.
--
-- 4) Agrega un índice (status, created_at) a ecom_meli_notifications para
--    que FindPending del procesador no haga scan completo cuando crezca el
--    backlog.
--
-- Ejecutar a mano contra la base de core-orchestrator.

-- 1) Payload resuelto de la notificación.
ALTER TABLE ecom_meli_notifications
  ADD COLUMN processed_payload LONGTEXT DEFAULT NULL AFTER raw_payload;

-- 2) Índice para FindPending.
ALTER TABLE ecom_meli_notifications
  ADD INDEX idx_meli_notif_status_created (status, created_at);

-- 3) Órdenes MELI (destino shadow del topic orders_v2).
--
-- El `id` interno NO es el id de MELI: ese es `meli_order_id`
-- (string "2000012345678"), único por conexión. Guardamos el id de MELI
-- como VARCHAR para no cerrar la puerta a cambios de formato.
CREATE TABLE ecom_meli_orders (
    id                       BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    connection_id            BIGINT UNSIGNED NOT NULL,
    meli_order_id            VARCHAR(64) NOT NULL,
    status                   VARCHAR(32) NOT NULL,
    status_detail            VARCHAR(255) DEFAULT NULL,
    date_created             TIMESTAMP NULL DEFAULT NULL,
    date_closed              TIMESTAMP NULL DEFAULT NULL,
    last_updated             TIMESTAMP NULL DEFAULT NULL,
    total_amount             DECIMAL(14,2) DEFAULT NULL,
    paid_amount              DECIMAL(14,2) DEFAULT NULL,
    currency_id              VARCHAR(8) DEFAULT NULL,
    buyer_meli_user_id       BIGINT UNSIGNED DEFAULT NULL,
    buyer_nickname           VARCHAR(100) DEFAULT NULL,
    buyer_first_name         VARCHAR(100) DEFAULT NULL,
    buyer_last_name          VARCHAR(100) DEFAULT NULL,
    buyer_email              VARCHAR(255) DEFAULT NULL,
    buyer_doc_type           VARCHAR(16) DEFAULT NULL,
    buyer_doc_number         VARCHAR(32) DEFAULT NULL,
    shipping_id              VARCHAR(64) DEFAULT NULL,
    pack_id                  VARCHAR(64) DEFAULT NULL,
    tags                     TEXT DEFAULT NULL,
    raw_payload              LONGTEXT NOT NULL,
    first_seen_notification_id BIGINT UNSIGNED DEFAULT NULL,
    last_seen_notification_id  BIGINT UNSIGNED DEFAULT NULL,
    created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at               TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uq_meli_order (connection_id, meli_order_id),
    KEY idx_meli_order_status (status, date_created),
    KEY idx_meli_order_shipping (shipping_id),
    CONSTRAINT fk_meli_order_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id)
);

CREATE TABLE ecom_meli_order_items (
    id                 BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    meli_order_pk      BIGINT UNSIGNED NOT NULL,
    item_meli_id       VARCHAR(64) NOT NULL,
    item_variation_id  VARCHAR(64) DEFAULT NULL,
    seller_sku         VARCHAR(255) DEFAULT NULL,
    title              VARCHAR(500) DEFAULT NULL,
    category_id        VARCHAR(64) DEFAULT NULL,
    unit_price         DECIMAL(14,2) DEFAULT NULL,
    full_unit_price    DECIMAL(14,2) DEFAULT NULL,
    quantity           INT DEFAULT NULL,
    currency_id        VARCHAR(8) DEFAULT NULL,
    sale_fee           DECIMAL(14,2) DEFAULT NULL,
    listing_type_id    VARCHAR(32) DEFAULT NULL,
    raw_payload        LONGTEXT NOT NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_meli_oi_order (meli_order_pk),
    KEY idx_meli_oi_sku (seller_sku),
    KEY idx_meli_oi_item (item_meli_id),
    CONSTRAINT fk_meli_oi_order FOREIGN KEY (meli_order_pk) REFERENCES ecom_meli_orders(id)
);

-- 4) Shipments (destino shadow del topic shipments).
CREATE TABLE ecom_meli_shipments (
    id                 BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    connection_id      BIGINT UNSIGNED NOT NULL,
    meli_shipment_id   VARCHAR(64) NOT NULL,
    meli_order_id      VARCHAR(64) DEFAULT NULL,
    status             VARCHAR(32) DEFAULT NULL,
    substatus          VARCHAR(64) DEFAULT NULL,
    mode               VARCHAR(32) DEFAULT NULL,
    tracking_number    VARCHAR(100) DEFAULT NULL,
    tracking_method    VARCHAR(100) DEFAULT NULL,
    service_id         VARCHAR(32) DEFAULT NULL,
    date_created       TIMESTAMP NULL DEFAULT NULL,
    last_updated       TIMESTAMP NULL DEFAULT NULL,
    raw_payload        LONGTEXT NOT NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at         TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uq_meli_shipment (connection_id, meli_shipment_id),
    KEY idx_meli_shipment_order (meli_order_id),
    CONSTRAINT fk_meli_shipment_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id)
);

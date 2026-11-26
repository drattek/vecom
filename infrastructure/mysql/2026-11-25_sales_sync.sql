-- Sincronización de ventas de MercadoLibre y Odoo (solo lectura). Ver ADR 0010.
--
-- Ejecutar a mano contra la base de core-orchestrator, ANTES de desplegar la
-- versión que incluye la sección "Ventas". Requiere que ya existan las tablas
-- de RBAC (2026-11-18_rbac_roles_permissions.sql).
--
-- Es re-ejecutable: las tablas usan IF NOT EXISTS y los permisos son
-- idempotentes. IF NOT EXISTS no modifica una tabla que ya exista.
--
-- Las fechas de negocio son DATETIME y las escribe la aplicación (el DSN usa
-- loc=Local). Un importe desconocido es NULL, no 0.

-- ---------------------------------------------------------------------------
-- 1) Ventas
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sales (
    id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    connection_id            BIGINT UNSIGNED NOT NULL,
    external_id              VARCHAR(64)  NOT NULL,
    order_number             VARCHAR(64)  NULL,
    pack_id                  VARCHAR(64)  NULL,

    -- Estados normalizados. El valor original de cada plataforma queda en
    -- external_status (venta), ecom_sale_payments y ecom_sale_shipments.
    order_status             VARCHAR(20)  NOT NULL DEFAULT 'unknown',
    payment_status           VARCHAR(20)  NOT NULL DEFAULT 'unknown',
    shipping_status          VARCHAR(20)  NOT NULL DEFAULT 'unknown',
    external_status          VARCHAR(64)  NULL,
    external_status_detail   VARCHAR(255) NULL,

    ordered_at               DATETIME     NOT NULL,
    closed_at                DATETIME     NULL,
    external_updated_at      DATETIME     NULL,

    -- Datos del cliente tal como estaban al sincronizar.
    customer_external_id     VARCHAR(64)  NULL,
    customer_name            VARCHAR(255) NULL,
    customer_nickname        VARCHAR(100) NULL,
    customer_email           VARCHAR(255) NULL,
    customer_phone           VARCHAR(64)  NULL,

    -- Datos fiscales.
    tax_id                   VARCHAR(32)  NULL,
    tax_name                 VARCHAR(255) NULL,
    tax_regime               VARCHAR(120) NULL,
    cfdi_use                 VARCHAR(120) NULL,
    billing_address          VARCHAR(600) NULL,

    -- Importes en la moneda original de la venta.
    currency_code            VARCHAR(8)    NULL,
    subtotal_amount          DECIMAL(14,2) NULL,
    discount_amount          DECIMAL(14,2) NULL,
    shipping_amount          DECIMAL(14,2) NULL,
    tax_amount               DECIMAL(14,2) NULL,
    total_amount             DECIMAL(14,2) NULL,
    paid_amount              DECIMAL(14,2) NULL,
    marketplace_fee_amount   DECIMAL(14,2) NULL,
    shipping_cost_seller     DECIMAL(14,2) NULL,

    -- Falta algún dato secundario; se reintenta en la siguiente corrida.
    is_incomplete            TINYINT(1)    NOT NULL DEFAULT 0,
    incomplete_reason        VARCHAR(500)  NULL,

    -- Respuestas originales, solo para auditoría. La API no las expone.
    raw_payload              LONGTEXT      NULL,
    billing_raw_payload      LONGTEXT      NULL,

    last_synced_at           DATETIME      NOT NULL,
    created_by               BIGINT UNSIGNED NULL,
    updated_by               BIGINT UNSIGNED NULL,
    created_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at               TIMESTAMP NULL DEFAULT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_connection_external (connection_id, external_id),
    KEY idx_sale_ordered_at (ordered_at),
    KEY idx_sale_connection_ordered (connection_id, ordered_at),
    KEY idx_sale_pack (connection_id, pack_id),
    KEY idx_sale_statuses (order_status, payment_status, shipping_status),
    CONSTRAINT fk_sale_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_sale_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 2) Productos vendidos
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sale_items (
    id                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    sale_id                BIGINT UNSIGNED NOT NULL,
    external_id            VARCHAR(120)  NOT NULL,
    external_product_id    VARCHAR(64)   NULL,
    external_variation_id  VARCHAR(64)   NULL,
    sku                    VARCHAR(255)  NULL,
    title                  VARCHAR(500)  NULL,
    quantity               DECIMAL(14,3) NOT NULL DEFAULT 0,
    unit_price             DECIMAL(14,2) NULL,
    list_unit_price        DECIMAL(14,2) NULL,
    discount_amount        DECIMAL(14,2) NULL,
    tax_amount             DECIMAL(14,2) NULL,
    total_amount           DECIMAL(14,2) NULL,
    fee_amount             DECIMAL(14,2) NULL,
    -- Vínculo con el catálogo local; NULL = sin vincular.
    product_id             BIGINT UNSIGNED NULL,
    match_method           VARCHAR(16)   NULL,  -- 'listing' | 'sku'
    created_by             BIGINT UNSIGNED NULL,
    updated_by             BIGINT UNSIGNED NULL,
    created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at             TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_item (sale_id, external_id),
    KEY idx_sale_item_sku (sku),
    KEY idx_sale_item_product (product_id),
    CONSTRAINT fk_sale_item_sale FOREIGN KEY (sale_id) REFERENCES ecom_sales(id),
    CONSTRAINT fk_sale_item_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_sale_item_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_item_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 3) Desglose del precio
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sale_charges (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    sale_id       BIGINT UNSIGNED NOT NULL,
    external_id   VARCHAR(160)  NOT NULL,
    -- De qué consulta salió ('order' | 'shipping'): si esa consulta falla, sus
    -- cargos guardados no se tocan.
    source        VARCHAR(16)   NOT NULL,
    charge_type   VARCHAR(16)   NOT NULL,  -- 'discount' | 'shipping' | 'tax' | 'fee'
    paid_by       VARCHAR(16)   NULL,      -- 'buyer' | 'seller' | 'marketplace'
    amount        DECIMAL(14,2) NOT NULL,
    description   VARCHAR(255)  NULL,
    created_by    BIGINT UNSIGNED NULL,
    updated_by    BIGINT UNSIGNED NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_charge (sale_id, external_id),
    CONSTRAINT fk_sale_charge_sale FOREIGN KEY (sale_id) REFERENCES ecom_sales(id),
    CONSTRAINT fk_sale_charge_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_charge_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 4) Pagos
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sale_payments (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    sale_id           BIGINT UNSIGNED NOT NULL,
    external_id       VARCHAR(64)   NOT NULL,
    method            VARCHAR(120)  NULL,
    payment_type      VARCHAR(64)   NULL,
    installments      INT           NULL,
    external_status   VARCHAR(64)   NULL,
    amount            DECIMAL(14,2) NULL,
    refunded_amount   DECIMAL(14,2) NULL,
    approved_at       DATETIME      NULL,
    created_by        BIGINT UNSIGNED NULL,
    updated_by        BIGINT UNSIGNED NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_payment (sale_id, external_id),
    CONSTRAINT fk_sale_payment_sale FOREIGN KEY (sale_id) REFERENCES ecom_sales(id),
    CONSTRAINT fk_sale_payment_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_payment_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 5) Envíos
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sale_shipments (
    id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    sale_id                 BIGINT UNSIGNED NOT NULL,
    external_id             VARCHAR(64)   NOT NULL,
    external_status         VARCHAR(64)   NULL,
    external_substatus      VARCHAR(64)   NULL,
    shipping_mode           VARCHAR(32)   NULL,
    logistic_type           VARCHAR(64)   NULL,
    carrier                 VARCHAR(160)  NULL,
    tracking_number         VARCHAR(120)  NULL,
    receiver_name           VARCHAR(255)  NULL,
    receiver_phone          VARCHAR(64)   NULL,
    receiver_address        VARCHAR(600)  NULL,
    dispatch_deadline       DATETIME      NULL,
    estimated_delivery_at   DATETIME      NULL,
    shipped_at              DATETIME      NULL,
    delivered_at            DATETIME      NULL,
    buyer_cost              DECIMAL(14,2) NULL,
    seller_cost             DECIMAL(14,2) NULL,
    raw_payload             LONGTEXT      NULL,
    created_by              BIGINT UNSIGNED NULL,
    updated_by              BIGINT UNSIGNED NULL,
    created_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at              TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_shipment (sale_id, external_id),
    CONSTRAINT fk_sale_shipment_sale FOREIGN KEY (sale_id) REFERENCES ecom_sales(id),
    CONSTRAINT fk_sale_shipment_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_shipment_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 6) Control del job, una fila por conexión
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_sale_sync_state (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    connection_id     BIGINT UNSIGNED NOT NULL,
    floor_at          DATETIME      NOT NULL,  -- ventas creadas antes no se importan
    cursor_at         DATETIME      NULL,      -- NULL = carga inicial pendiente
    lock_token        VARCHAR(64)   NULL,
    locked_until      DATETIME      NULL,
    last_started_at   DATETIME      NULL,
    last_finished_at  DATETIME      NULL,
    last_success_at   DATETIME      NULL,
    last_error        VARCHAR(1000) NULL,
    last_saved_count  INT           NOT NULL DEFAULT 0,
    last_failed_count INT           NOT NULL DEFAULT 0,
    created_by        BIGINT UNSIGNED NULL,
    updated_by        BIGINT UNSIGNED NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sale_sync_connection (connection_id),
    CONSTRAINT fk_sale_sync_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_sale_sync_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_sale_sync_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 7) Permisos (RBAC, ADR 0009). Ningún rol existente los recibe solo: se
--    asignan en Configuración → Roles. admin tiene acceso por defecto.
-- ---------------------------------------------------------------------------
INSERT INTO ecom_permissions (resource, action, description) VALUES
    ('sales', 'view', 'Ver las ventas y su detalle, incluidos los datos del cliente'),
    ('sales', 'sync', 'Ejecutar "Sincronizar ahora" en la sección de ventas')
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- ---------------------------------------------------------------------------
-- Opcional: limitar una conexión de Odoo a un solo sitio web. Necesario si dos
-- conexiones apuntan a la misma instancia. También se puede capturar desde
-- Configuración → Conexiones. Sustituir los tres valores.
-- ---------------------------------------------------------------------------
-- INSERT INTO ecom_connection_settings (connection_id, key_name, value, is_encrypted, created_by)
-- VALUES (<connection_id>, 'sales_website_id', '<website_id>', 0, <user_id>);

-- Verificación rápida:
--   SHOW TABLES LIKE 'ecom_sale%';
--   SELECT * FROM ecom_permissions WHERE resource = 'sales';

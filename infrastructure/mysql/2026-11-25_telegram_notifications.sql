-- Notificaciones por Telegram. Ver ADR 0012.
--
-- Ejecutar a mano contra la base de core-orchestrator, ANTES de desplegar la
-- versión que incluye las notificaciones.
--
-- Es re-ejecutable: las tablas usan IF NOT EXISTS y las filas iniciales son
-- idempotentes. IF NOT EXISTS no modifica una tabla que ya exista.
--
-- Las fechas de negocio son DATETIME y las escribe la aplicación (el DSN usa
-- loc=Local). created_by / updated_by son NULL cuando escribe un proceso
-- automático.
--
-- No agrega permisos: la configuración del bot es solo para admin (ADR 0009) y
-- cada usuario administra únicamente su propio vínculo y sus preferencias.

-- ---------------------------------------------------------------------------
-- 1) Configuración global (una sola fila)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_notification_settings (
    id                        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    -- Siempre 1: el índice único impide una segunda fila.
    singleton_key             TINYINT UNSIGNED NOT NULL DEFAULT 1,
    -- Cifrado con AES-256-GCM y CONNECTION_CREDENTIALS_ENCRYPTION_KEY (mismo
    -- formato que las credenciales de conexión). Nunca en texto plano.
    bot_token_encrypted       TEXT         NULL,
    -- Se obtiene de Telegram (getMe) al guardar el token.
    bot_username              VARCHAR(64)  NULL,
    is_enabled                TINYINT(1)   NOT NULL DEFAULT 0,
    -- Hora local del resumen diario, la misma para todos los usuarios.
    daily_summary_time        TIME         NOT NULL DEFAULT '08:00:00',
    timezone                  VARCHAR(64)  NOT NULL DEFAULT 'America/Mexico_City',
    -- Horas sin una sincronización completada antes de avisar. El ERP se
    -- sincroniza a diario, pero procesar todo puede demorar: el margen de 12
    -- horas evita alertas falsas.
    erp_silence_hours         SMALLINT UNSIGNED NOT NULL DEFAULT 36,
    -- Cada cuánto se comprueban las conexiones de marketplace.
    connection_check_minutes  SMALLINT UNSIGNED NOT NULL DEFAULT 60,
    -- Último update_id de getUpdates procesado. Vuelve a 0 al cambiar de bot.
    telegram_update_offset    BIGINT       NOT NULL DEFAULT 0,
    created_by                BIGINT UNSIGNED NULL,
    updated_by                BIGINT UNSIGNED NULL,
    created_at                TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at                TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_notification_settings_singleton (singleton_key),
    CONSTRAINT fk_notif_settings_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_settings_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 2) Vínculo de cada usuario con su Telegram (una fila por usuario)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_user_telegram_links (
    id                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id                BIGINT UNSIGNED NOT NULL,
    -- 'blocked' = el usuario bloqueó el bot.
    status                 ENUM('pending','linked','blocked','unlinked') NOT NULL DEFAULT 'pending',
    -- Interruptor general del usuario; no borra sus preferencias.
    notifications_enabled  TINYINT(1)   NOT NULL DEFAULT 1,
    -- NULL mientras no haya vínculo (al desvincular se limpia).
    chat_id                BIGINT       NULL,
    telegram_username      VARCHAR(64)  NULL,
    telegram_name          VARCHAR(255) NULL,
    -- Bot con el que se vinculó: si se cambia de bot, hay que vincular de nuevo.
    bot_username           VARCHAR(64)  NULL,
    -- SHA-256 (hex) del código de un solo uso; el código no se guarda.
    link_code_hash         CHAR(64)     NULL,
    link_code_expires_at   DATETIME     NULL,
    linked_at              DATETIME     NULL,
    created_by             BIGINT UNSIGNED NULL,
    updated_by             BIGINT UNSIGNED NULL,
    created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at             TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_telegram_link_user (user_id),
    -- Un chat solo puede pertenecer a un usuario (los NULL no chocan).
    UNIQUE KEY uq_telegram_link_chat (chat_id),
    UNIQUE KEY uq_telegram_link_code (link_code_hash),
    CONSTRAINT fk_telegram_link_user FOREIGN KEY (user_id) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_telegram_link_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_telegram_link_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 3) Avisos que cada usuario quiere recibir. Sin fila = apagado.
-- ---------------------------------------------------------------------------
-- notification_type (el catálogo y su permiso viven en el código):
--   'erp_sync_failure'       falla, falla parcial o silencio del ERP
--   'connection_down'        conexión caída o token vencido
--   'publication_errors'     errores al publicar o resincronizar
--   'stock_pause_summary'    resumen diario de pausas por falta de stock
--   'meli_listing_closed'    cierres definitivos de MercadoLibre
--   'bulk_process_finished'  fin de importación o actualización masiva propia
CREATE TABLE IF NOT EXISTS ecom_notification_preferences (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id            BIGINT UNSIGNED NOT NULL,
    notification_type  VARCHAR(40) NOT NULL,
    is_enabled         TINYINT(1)  NOT NULL DEFAULT 0,
    created_by         BIGINT UNSIGNED NULL,
    updated_by         BIGINT UNSIGNED NULL,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at         TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_notification_pref (user_id, notification_type),
    KEY idx_notification_pref_type (notification_type, is_enabled),
    CONSTRAINT fk_notif_pref_user FOREIGN KEY (user_id) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_pref_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_pref_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 4) Cola e historial de envíos, una fila por destinatario
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_notification_deliveries (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id              BIGINT UNSIGNED NOT NULL,
    notification_type    VARCHAR(40)   NOT NULL,  -- también 'test'
    -- Identifica el hecho avisado; impide encolarlo dos veces al mismo usuario.
    dedupe_key           VARCHAR(190)  NOT NULL,
    message              TEXT          NOT NULL,
    -- Archivo adjunto cuando el detalle no cabe en un mensaje.
    attachment_name      VARCHAR(255)  NULL,
    attachment_content   MEDIUMBLOB    NULL,
    status               ENUM('pending','sending','sent','failed','cancelled') NOT NULL DEFAULT 'pending',
    attempts             INT UNSIGNED  NOT NULL DEFAULT 0,
    -- Respeta el retry_after que indique Telegram.
    next_attempt_at      DATETIME      NULL,
    -- Un 'sending' con el bloqueo vencido se puede retomar.
    locked_until         DATETIME      NULL,
    last_error           VARCHAR(1000) NULL,      -- sin el token del bot
    telegram_message_id  BIGINT        NULL,
    sent_at              DATETIME      NULL,
    created_by           BIGINT UNSIGNED NULL,
    updated_by           BIGINT UNSIGNED NULL,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at           TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_notification_delivery (user_id, dedupe_key),
    KEY idx_notification_delivery_queue (status, next_attempt_at, id),
    CONSTRAINT fk_notif_delivery_user FOREIGN KEY (user_id) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_delivery_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_delivery_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 5) Incidentes abiertos y resueltos: avisar una vez y avisar la recuperación
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_notification_incidents (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    -- 'erp_failed' | 'erp_partial' | 'erp_silence' | 'connection_down'
    -- | 'publication_error'
    incident_type  VARCHAR(40)  NOT NULL,
    -- p. ej. 'erp_silence:stock', 'connection_down:3',
    -- 'publication_error:<product_id>:<connection_id>'
    incident_key   VARCHAR(190) NOT NULL,
    status         ENUM('open','resolved') NOT NULL DEFAULT 'open',
    -- SHA-256 del error normalizado: si cambia, se vuelve a avisar.
    signature      CHAR(64)     NULL,
    connection_id  BIGINT UNSIGNED NULL,
    product_id     BIGINT UNSIGNED NULL,
    message        TEXT         NULL,
    opened_at      DATETIME     NOT NULL,
    last_seen_at   DATETIME     NOT NULL,
    resolved_at    DATETIME     NULL,
    -- Solo un incidente abierto por clave.
    open_key       VARCHAR(190) GENERATED ALWAYS AS (
        CASE WHEN status = 'open' AND deleted_at IS NULL THEN incident_key END
    ) VIRTUAL,
    created_by     BIGINT UNSIGNED NULL,
    updated_by     BIGINT UNSIGNED NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at     TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_notification_incident_open (open_key),
    KEY idx_notification_incident_key (incident_key, id),
    KEY idx_notification_incident_type (incident_type, status),
    CONSTRAINT fk_notif_incident_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_notif_incident_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_notif_incident_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_notif_incident_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 6) Última actividad de cada flujo del ERP (base del aviso por silencio)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_erp_sync_state (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    feed_code          VARCHAR(40)   NOT NULL,  -- 'products' | 'stock' | 'nissan_stock'
    -- NULL = aún sin registro; el silencio se cuenta desde created_at.
    last_completed_at  DATETIME      NULL,
    last_failed_at     DATETIME      NULL,
    last_error         VARCHAR(1000) NULL,
    last_total_count   INT UNSIGNED  NOT NULL DEFAULT 0,
    last_failed_count  INT UNSIGNED  NOT NULL DEFAULT 0,
    created_by         BIGINT UNSIGNED NULL,
    updated_by         BIGINT UNSIGNED NULL,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at         TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_erp_sync_state_feed (feed_code),
    CONSTRAINT fk_erp_sync_state_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_erp_sync_state_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 7) Publicaciones cerradas en MercadoLibre y su motivo
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_channel_listing_closures (
    id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    connection_id           BIGINT UNSIGNED NOT NULL,
    product_id              BIGINT UNSIGNED NOT NULL,
    channel_product_map_id  BIGINT UNSIGNED NOT NULL,
    external_id             VARCHAR(100) NOT NULL,
    -- Valores originales de MercadoLibre.
    external_status         VARCHAR(32)  NULL,
    external_sub_status     VARCHAR(255) NULL,
    -- Solo se avisan los 'definitive'. 'system' = no se atribuye a
    -- MercadoLibre (la cerró este sistema o el vendedor).
    closure_kind            ENUM('definitive','fixable','system') NOT NULL,
    -- 'pending' = falta consultar (se reintenta);
    -- 'unavailable' = MercadoLibre no informa motivo.
    reason_status           ENUM('pending','found','unavailable') NOT NULL DEFAULT 'pending',
    moderation_name         VARCHAR(120) NULL,
    reason                  TEXT         NULL,
    remedy                  TEXT         NULL,
    detected_at             DATETIME     NOT NULL,
    notified_at             DATETIME     NULL,
    -- Se llena si la publicación deja de estar cerrada.
    reopened_at             DATETIME     NULL,
    -- Un solo cierre vigente por publicación.
    active_key              VARCHAR(160) GENERATED ALWAYS AS (
        CASE WHEN reopened_at IS NULL AND deleted_at IS NULL
             THEN CONCAT(connection_id, ':', external_id) END
    ) VIRTUAL,
    created_by              BIGINT UNSIGNED NULL,
    updated_by              BIGINT UNSIGNED NULL,
    created_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at              TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_listing_closure_active (active_key),
    KEY idx_listing_closure_pending (closure_kind, notified_at, id),
    KEY idx_listing_closure_product (product_id, connection_id),
    CONSTRAINT fk_listing_closure_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_listing_closure_product FOREIGN KEY (product_id) REFERENCES ecom_products(id),
    CONSTRAINT fk_listing_closure_map FOREIGN KEY (channel_product_map_id) REFERENCES ecom_channel_product_map(id),
    CONSTRAINT fk_listing_closure_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_listing_closure_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 8) Ejecuciones de las tareas con horario fijo (no repetir tras un reinicio)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ecom_scheduled_job_runs (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    -- 'stock_pause_summary' | 'meli_closures:<connection_id>'
    -- | 'meli_baseline:<connection_id>' (primera revisión de la conexión)
    job_code       VARCHAR(40)   NOT NULL,
    -- Horario que correspondía, en la zona horaria configurada.
    scheduled_for  DATETIME      NOT NULL,
    status         ENUM('running','done','failed') NOT NULL DEFAULT 'running',
    -- Periodo que cubrió la corrida (resumen de pausas).
    window_from    DATETIME      NULL,
    window_to      DATETIME      NULL,
    started_at     DATETIME      NOT NULL,
    finished_at    DATETIME      NULL,
    locked_until   DATETIME      NULL,
    last_error     VARCHAR(1000) NULL,
    created_by     BIGINT UNSIGNED NULL,
    updated_by     BIGINT UNSIGNED NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at     TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_scheduled_job_run (job_code, scheduled_for),
    CONSTRAINT fk_job_run_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_job_run_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- ---------------------------------------------------------------------------
-- 9) Filas iniciales. No modifican filas que ya existan.
-- ---------------------------------------------------------------------------
-- La configuración nace apagada y sin token: se captura desde el dashboard.
INSERT INTO ecom_notification_settings (singleton_key) VALUES (1)
ON DUPLICATE KEY UPDATE singleton_key = singleton_key;

INSERT INTO ecom_erp_sync_state (feed_code) VALUES
    ('products'),
    ('stock'),
    ('nissan_stock')
ON DUPLICATE KEY UPDATE feed_code = feed_code;

-- Verificación rápida:
--   SHOW TABLES LIKE 'ecom_notification%';
--   SHOW TABLES LIKE 'ecom_user_telegram_links';
--   SHOW TABLES LIKE 'ecom_erp_sync_state';
--   SHOW TABLES LIKE 'ecom_channel_listing_closures';
--   SHOW TABLES LIKE 'ecom_scheduled_job_runs';
--   SELECT * FROM ecom_notification_settings;
--   SELECT * FROM ecom_erp_sync_state;

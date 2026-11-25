-- Respaldos automáticos de la configuración de MercadoLibre de una conexión
-- (ADR 0011). core-orchestrator guarda uno antes de editar la aplicación, de
-- vincular una cuenta o de restaurar.
--
-- Ejecutar a mano contra la base de core-orchestrator ANTES de desplegar la
-- versión que la usa: sin esta tabla, "Editar aplicación", "Vincular cuenta" y
-- "Restaurar" fallan sin cambiar nada (la renovación de tokens no la usa).
--
-- Es re-ejecutable (IF NOT EXISTS). No toca los respaldos manuales.

CREATE TABLE IF NOT EXISTS ecom_connection_settings_backups (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    connection_id  BIGINT UNSIGNED NOT NULL,
    -- 'app_update' | 'account_link' | 'restore'
    reason         VARCHAR(32)     NOT NULL,
    -- Copia sin cifrar del app_id (no es secreto), para mostrarlo en la lista.
    app_id         VARCHAR(64)     NULL,
    -- Ajustes respaldados: un JSON cifrado completo (AES-GCM, con la misma
    -- CONNECTION_CREDENTIALS_ENCRYPTION_KEY que ecom_connection_settings).
    payload        LONGTEXT        NOT NULL,
    created_by     BIGINT UNSIGNED NOT NULL,
    updated_by     BIGINT UNSIGNED NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    -- Solo se muestran los 5 más recientes por conexión; los demás se ocultan.
    deleted_at     TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_conn_settings_backup (connection_id, deleted_at, id),
    CONSTRAINT fk_conn_backup_connection FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    CONSTRAINT fk_conn_backup_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_conn_backup_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- Verificación rápida:
--   SHOW TABLES LIKE 'ecom_connection_settings_backups';

-- Credenciales de acceso de un disco de almacenamiento (hoy: Azure Blob Storage
-- para las imágenes que se importan desde las publicaciones de MercadoLibre).
--
-- Una fila por clave (account_name, account_key, container_name). Todos los
-- valores se guardan CIFRADOS (AES-256-GCM, con CONNECTION_CREDENTIALS_ENCRYPTION_KEY,
-- el mismo formato que ecom_connection_credentials), así que nunca se leen ni
-- se escriben a mano: se cargan con
--   PUT /api/storage-disks/{id}/azure-credentials
--   { "accountName": "...", "accountKey": "...", "containerName": "..." }
-- Cambiar de contenedor es volver a llamar ese endpoint.
--
-- Ejecutar a mano contra la base de core-orchestrator. Además hay que tener un
-- disco con code 'AZURE_BLOB' en ecom_storage_disks (ejemplo al final).

CREATE TABLE ecom_storage_disk_credentials (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    disk_id BIGINT UNSIGNED NOT NULL,
    key_name VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    is_encrypted BOOLEAN DEFAULT 1,
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    UNIQUE KEY uq_storage_disk_credential (disk_id, key_name),
    CONSTRAINT fk_storage_disk_credential_disk FOREIGN KEY (disk_id) REFERENCES ecom_storage_disks(id),
    CONSTRAINT fk_storage_disk_credential_created FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_storage_disk_credential_updated FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

-- Ejemplo del disco (ajustar base_url a <cuenta>.blob.core.windows.net/<contenedor>):
-- INSERT INTO ecom_storage_disks (name, code, base_url, bucket, is_public, created_by)
-- VALUES ('Azure Blob', 'AZURE_BLOB', 'https://<cuenta>.blob.core.windows.net/<contenedor>', '<contenedor>', 1, 1);

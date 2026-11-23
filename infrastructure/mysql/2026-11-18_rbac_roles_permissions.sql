-- RBAC del admin-dashboard: roles, catálogo de permisos, permisos por rol y
-- excepciones por usuario. Ver ADR 0009.
--
-- Regla: ecom_api_user.role = 'admin' tiene acceso total y NO pasa por estas
-- tablas (cortocircuito en backend y frontend). role_id solo aplica al resto.
--
-- Permisos efectivos = (permisos del rol  ∪  grants del usuario) − denies del usuario

CREATE TABLE IF NOT EXISTS ecom_roles (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code         VARCHAR(50)  NOT NULL,
    name         VARCHAR(100) NOT NULL,
    description  VARCHAR(255) NULL,
    created_by   BIGINT UNSIGNED NULL,
    updated_by   BIGINT UNSIGNED NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at   TIMESTAMP NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_ecom_roles_code (code),
    CONSTRAINT fk_role_created_by FOREIGN KEY (created_by) REFERENCES ecom_api_user(id),
    CONSTRAINT fk_role_updated_by FOREIGN KEY (updated_by) REFERENCES ecom_api_user(id)
);

CREATE TABLE IF NOT EXISTS ecom_permissions (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    resource     VARCHAR(50) NOT NULL,
    action       VARCHAR(30) NOT NULL,
    description  VARCHAR(255) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_ecom_permissions (resource, action)
);

CREATE TABLE IF NOT EXISTS ecom_role_permissions (
    role_id        BIGINT UNSIGNED NOT NULL,
    permission_id  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_role_perm_role       FOREIGN KEY (role_id)       REFERENCES ecom_roles(id),
    CONSTRAINT fk_role_perm_permission FOREIGN KEY (permission_id) REFERENCES ecom_permissions(id)
);

CREATE TABLE IF NOT EXISTS ecom_user_permissions (
    user_id        BIGINT UNSIGNED NOT NULL,
    permission_id  BIGINT UNSIGNED NOT NULL,
    effect         ENUM('grant','deny') NOT NULL DEFAULT 'grant',
    created_by     BIGINT UNSIGNED NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, permission_id),
    CONSTRAINT fk_user_perm_user       FOREIGN KEY (user_id)       REFERENCES ecom_api_user(id),
    CONSTRAINT fk_user_perm_permission FOREIGN KEY (permission_id) REFERENCES ecom_permissions(id),
    CONSTRAINT fk_user_perm_created_by FOREIGN KEY (created_by)    REFERENCES ecom_api_user(id)
);

-- NO idempotente: ejecutar una sola vez (falla si la columna ya existe).
ALTER TABLE ecom_api_user
    ADD COLUMN role_id BIGINT UNSIGNED NULL,
    ADD CONSTRAINT fk_api_user_role FOREIGN KEY (role_id) REFERENCES ecom_roles(id);

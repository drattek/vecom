-- ============================================================================
-- Normaliza los nombres de sucursales y almacenes: quita los espacios (U+0020) del
-- inicio y del final. Es lo que ahora hace el backend (NormalizeLocationName) antes de
-- buscar o guardar; sin esto, un registro guardado como 'LEON ' no se encontraría con
-- la búsqueda 'LEON' y el sync crearía una sucursal duplicada.
--
-- Seguridad:
--   * Solo toca espacios del inicio/final (TRIM de MySQL). No cambia mayúsculas,
--     acentos ni espacios internos.
--   * Si al quitar los espacios DOS registros ACTIVOS quedarían con el mismo nombre
--     (sucursal repetida, o almacén repetido dentro de la misma sucursal), esos
--     registros NO se tocan y salen en el reporte final para resolverlos a mano:
--     renombrar no debe crear duplicados.
--   * Los registros dados de baja siempre se normalizan (no participan en búsquedas).
--   * Respaldo previo en bak_20261123_names_*. Corre todo en una sola sesión.
-- Idempotente: se puede volver a correr.
-- ============================================================================


/* ===================================================================== */
/* PARTE A — DIAGNÓSTICO (solo lectura)                                  */
/* ===================================================================== */

-- A1. Nombres con espacios al inicio/final (el largo se compara con CHAR_LENGTH porque
--     algunas collations ignoran los espacios finales al comparar con "=").
SELECT 'SUCURSAL' AS tipo, id, CONCAT('[', name, ']') AS nombre, deleted_at
FROM ecom_branches
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))
UNION ALL
SELECT 'ALMACEN', id, CONCAT('[', name, ']'), deleted_at
FROM ecom_warehouses
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name));

-- A2. Colisiones que causaría la normalización (registros ACTIVOS que quedarían iguales).
SELECT 'SUCURSAL' AS tipo, TRIM(name) AS nombre_normalizado,
       COUNT(*) AS filas, GROUP_CONCAT(id ORDER BY id) AS ids
FROM ecom_branches
WHERE deleted_at IS NULL
GROUP BY TRIM(name)
HAVING COUNT(*) > 1 AND SUM(CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))) > 0
UNION ALL
SELECT 'ALMACEN', CONCAT(g.n, ' (sucursal ', g.branch_id, ')'), g.filas, g.ids
FROM (
    SELECT TRIM(name) AS n, branch_id, COUNT(*) AS filas, GROUP_CONCAT(id ORDER BY id) AS ids
    FROM ecom_warehouses
    WHERE deleted_at IS NULL
    GROUP BY TRIM(name), branch_id
    HAVING COUNT(*) > 1 AND SUM(CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))) > 0
) g;


/* ===================================================================== */
/* PARTE B — NORMALIZACIÓN                                               */
/* ===================================================================== */

-- B0. Respaldo de los nombres originales (no se pisa si ya existe).
CREATE TABLE IF NOT EXISTS bak_20261123_names_branches   AS SELECT id, name FROM ecom_branches;
CREATE TABLE IF NOT EXISTS bak_20261123_names_warehouses AS SELECT id, name, branch_id FROM ecom_warehouses;

START TRANSACTION;

-- B1. Registros ACTIVOS que chocarían al normalizar: se excluyen de la actualización.
DROP TEMPORARY TABLE IF EXISTS tmp_branch_conflict;
CREATE TEMPORARY TABLE tmp_branch_conflict AS
SELECT b.id
FROM ecom_branches b
JOIN (
    SELECT TRIM(name) AS n
    FROM ecom_branches
    WHERE deleted_at IS NULL
    GROUP BY TRIM(name)
    HAVING COUNT(*) > 1 AND SUM(CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))) > 0
) g ON g.n = TRIM(b.name)
WHERE b.deleted_at IS NULL;

DROP TEMPORARY TABLE IF EXISTS tmp_warehouse_conflict;
CREATE TEMPORARY TABLE tmp_warehouse_conflict AS
SELECT w.id
FROM ecom_warehouses w
JOIN (
    SELECT TRIM(name) AS n, branch_id
    FROM ecom_warehouses
    WHERE deleted_at IS NULL
    GROUP BY TRIM(name), branch_id
    HAVING COUNT(*) > 1 AND SUM(CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))) > 0
) g ON g.n = TRIM(w.name) AND g.branch_id = w.branch_id
WHERE w.deleted_at IS NULL;

-- B2. Normalizar.
UPDATE ecom_branches
SET name = TRIM(name)
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))
  AND id NOT IN (SELECT c.id FROM tmp_branch_conflict c);

UPDATE ecom_warehouses
SET name = TRIM(name)
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name))
  AND id NOT IN (SELECT c.id FROM tmp_warehouse_conflict c);

COMMIT;

-- B3. REPORTE: lo que quedó sin normalizar por colisión. Debe estar vacío; si no, hay
--     sucursales/almacenes duplicados que hay que fusionar a mano antes de seguir.
SELECT 'SUCURSAL SIN NORMALIZAR (colisión)' AS reporte, id, CONCAT('[', name, ']') AS nombre
FROM ecom_branches
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name)) AND deleted_at IS NULL
UNION ALL
SELECT 'ALMACEN SIN NORMALIZAR (colisión)', id, CONCAT('[', name, '] sucursal ', branch_id)
FROM ecom_warehouses
WHERE CHAR_LENGTH(name) <> CHAR_LENGTH(TRIM(name)) AND deleted_at IS NULL;

-- Para revertir: restaurar los nombres desde bak_20261123_names_branches /
-- bak_20261123_names_warehouses.


/* ===================================================================== */
/* PARTE C — BLINDAJE EN BASE DE DATOS (solo si el reporte B3 salió vacío) */
/* ===================================================================== */
-- Impide guardar nombres con espacios al inicio/final. Valida las filas existentes, así
-- que falla si B3 dejó registros sin normalizar (a propósito). Requiere MySQL >= 8.0.16.

ALTER TABLE ecom_branches
    ADD CONSTRAINT chk_ecom_branches_name_trimmed CHECK (CHAR_LENGTH(name) = CHAR_LENGTH(TRIM(name)));

ALTER TABLE ecom_warehouses
    ADD CONSTRAINT chk_ecom_warehouses_name_trimmed CHECK (CHAR_LENGTH(name) = CHAR_LENGTH(TRIM(name)));

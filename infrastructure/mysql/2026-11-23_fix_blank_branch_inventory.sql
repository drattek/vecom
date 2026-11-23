-- ============================================================================
-- Corrección completa del inventario colgado de sucursales SIN NOMBRE
-- ("Sin sucursal"). Reemplaza a los scripts 2026-11-23_fix_blank_branch_stock.sql y
-- 2026-11-23_delete_blank_branch_inventory.sql: corre SOLO este.
--
-- Causa: el sync del ERP creaba una sucursal con nombre vacío cuando dyn.ItemInventLocation
-- traía Name NULL/vacío, y volvía a crear ahí el almacén (p. ej. LEN) que ya existía bajo
-- la sucursal real (p. ej. LEON). El código ya no lo permite (resolveErpBranch y las
-- validaciones de los repositorios); este script limpia lo ya creado.
--
-- Qué hace (en una sola transacción, con respaldo previo):
--   1. Fusiona el stock: la fila del almacén real se queda con la cantidad MÁS RECIENTE
--      (la que el sync venía alimentando bajo la sucursal sin nombre). Cada cambio de
--      cantidad queda registrado con un movimiento 'adjustment'.
--   2. Stock que solo existía bajo la sucursal sin nombre, en un almacén con destino
--      único -> se reasigna al almacén real (con su movimiento 'adjustment').
--   3. Elimina TODOS los movimientos de esas sucursales/almacenes (soft delete).
--   4. Elimina el stock restante bajo sucursal sin nombre (DELETE físico: la tabla no
--      tiene deleted_at). Aquí se pierde el stock de almacenes sin destino único.
--   5. Da de baja (soft delete) esos almacenes y sucursales.
--   6. Verifica que no quede nada; si falla, el script se detiene antes del COMMIT.
--   7. (Parte C, después del COMMIT) blinda la tabla con un CHECK.
--
-- "Sin nombre" = sin ninguna letra ni dígito (vacío, espacios, invisibles).
-- Respaldo: bak_20261123_* (si ya existen de una corrida anterior, conservan el estado
-- ORIGINAL y no se pisan). Corre TODO en una sola sesión (usa tablas temporales).
-- Requiere MySQL >= 8.0.16 (CHECK).
-- ============================================================================


/* ===================================================================== */
/* PARTE A — DIAGNÓSTICO (solo lectura; puedes correrla sola)            */
/* ===================================================================== */

-- A1. Sucursales sin nombre y qué cuelga de ellas.
SELECT b.id AS branch_id, CONCAT('[', COALESCE(b.name, 'NULL'), ']') AS nombre, b.deleted_at,
       (SELECT COUNT(*) FROM ecom_warehouses w WHERE w.branch_id = b.id AND w.deleted_at IS NULL) AS almacenes_activos,
       (SELECT COUNT(*) FROM ecom_product_stock ps WHERE ps.branch_id = b.id) AS filas_stock
FROM ecom_branches b
WHERE COALESCE(b.name, '') NOT REGEXP '[[:alnum:]]';

-- A2. Destino de cada almacén sin sucursal. candidatos = 1 -> se fusiona; 0 o >1 -> su
--     stock se ELIMINA (revisa unidades antes de continuar).
SELECT wb.id AS almacen_sin_sucursal, wb.name AS almacen,
       COUNT(DISTINCT wr.id) AS candidatos,
       GROUP_CONCAT(DISTINCT CONCAT(br.name, ' (wh ', wr.id, ')') SEPARATOR ', ') AS destino,
       (SELECT COALESCE(SUM(ps.available_qty), 0) FROM ecom_product_stock ps WHERE ps.warehouse_id = wb.id) AS unidades
FROM ecom_warehouses wb
JOIN ecom_branches bb ON bb.id = wb.branch_id AND COALESCE(bb.name, '') NOT REGEXP '[[:alnum:]]'
LEFT JOIN ecom_warehouses wr ON wr.name = wb.name AND wr.deleted_at IS NULL AND wr.id <> wb.id
LEFT JOIN ecom_branches br ON br.id = wr.branch_id AND br.deleted_at IS NULL AND COALESCE(br.name, '') REGEXP '[[:alnum:]]'
WHERE wb.deleted_at IS NULL
GROUP BY wb.id, wb.name;


/* ===================================================================== */
/* PARTE B — CORRECCIÓN (una sola ejecución, termina en COMMIT)          */
/* ===================================================================== */

-- B0. Respaldos.
CREATE TABLE IF NOT EXISTS bak_20261123_product_stock    AS SELECT * FROM ecom_product_stock;
CREATE TABLE IF NOT EXISTS bak_20261123_stock_movements  AS SELECT * FROM ecom_stock_movements;
CREATE TABLE IF NOT EXISTS bak_20261123_warehouses       AS SELECT * FROM ecom_warehouses;
CREATE TABLE IF NOT EXISTS bak_20261123_branches         AS SELECT * FROM ecom_branches;

START TRANSACTION;

-- B1. Sucursales sin nombre (aunque ya estén de baja), sus almacenes y el mapa
--     almacén-sin-sucursal -> almacén real (solo cuando hay EXACTAMENTE un destino).
DROP TEMPORARY TABLE IF EXISTS tmp_blank_branch;
CREATE TEMPORARY TABLE tmp_blank_branch AS
SELECT b.id
FROM ecom_branches b
WHERE COALESCE(b.name, '') NOT REGEXP '[[:alnum:]]';

DROP TEMPORARY TABLE IF EXISTS tmp_blank_wh;
CREATE TEMPORARY TABLE tmp_blank_wh AS
SELECT w.id
FROM ecom_warehouses w
WHERE w.branch_id IN (SELECT t.id FROM tmp_blank_branch t);

DROP TEMPORARY TABLE IF EXISTS tmp_wh_map;
CREATE TEMPORARY TABLE tmp_wh_map AS
SELECT wb.id             AS blank_wh_id,
       MIN(wr.id)        AS real_wh_id,
       MIN(wr.branch_id) AS real_branch_id
FROM ecom_warehouses wb
JOIN ecom_branches bb ON bb.id = wb.branch_id AND bb.deleted_at IS NULL AND COALESCE(bb.name, '') NOT REGEXP '[[:alnum:]]'
JOIN ecom_warehouses wr ON wr.name = wb.name AND wr.deleted_at IS NULL AND wr.id <> wb.id
JOIN ecom_branches br ON br.id = wr.branch_id AND br.deleted_at IS NULL AND COALESCE(br.name, '') REGEXP '[[:alnum:]]'
WHERE wb.deleted_at IS NULL
GROUP BY wb.id
HAVING COUNT(DISTINCT wr.id) = 1;

-- B2. Productos que existen en ambos almacenes: se decide la cantidad MÁS RECIENTE.
--     Recencia = fecha del último movimiento de cada fila (o last_sync_at si no tiene).
--     En empate gana la fila bajo sucursal sin nombre, que es la que el sync alimentaba.
DROP TEMPORARY TABLE IF EXISTS tmp_stock_pick;
CREATE TEMPORARY TABLE tmp_stock_pick AS
SELECT real_ps.id                AS real_stock_id,
       real_ps.product_id        AS product_id,
       m.real_branch_id          AS branch_id,
       m.real_wh_id              AS warehouse_id,
       real_ps.available_qty     AS old_qty,
       real_ps.last_sync_at      AS real_sync,
       blank_ps.last_sync_at     AS blank_sync,
       CASE
           WHEN COALESCE(
                    (SELECT MAX(sm.created_at) FROM ecom_stock_movements sm
                      WHERE sm.product_id = blank_ps.product_id AND sm.warehouse_id = m.blank_wh_id
                        AND sm.deleted_at IS NULL),
                    blank_ps.last_sync_at, '1970-01-02')
             >= COALESCE(
                    (SELECT MAX(sm.created_at) FROM ecom_stock_movements sm
                      WHERE sm.product_id = real_ps.product_id AND sm.warehouse_id = m.real_wh_id
                        AND sm.deleted_at IS NULL),
                    real_ps.last_sync_at, '1970-01-02')
           THEN blank_ps.available_qty
           ELSE real_ps.available_qty
       END                       AS new_qty
FROM ecom_product_stock real_ps
JOIN tmp_wh_map m ON m.real_wh_id = real_ps.warehouse_id
JOIN ecom_product_stock blank_ps ON blank_ps.warehouse_id = m.blank_wh_id
                                AND blank_ps.product_id   = real_ps.product_id;

UPDATE ecom_product_stock real_ps
JOIN tmp_stock_pick p ON p.real_stock_id = real_ps.id
SET real_ps.available_qty = p.new_qty,
    real_ps.last_sync_at  = GREATEST(COALESCE(p.real_sync, p.blank_sync), COALESCE(p.blank_sync, p.real_sync))
WHERE p.real_stock_id IS NOT NULL;

-- Cada cambio de cantidad deja su rastro (los movimientos viejos se eliminan en B5).
INSERT INTO ecom_stock_movements (product_id, branch_id, warehouse_id, movement_type,
                                  quantity_before, quantity_change, quantity_after, updated_by)
SELECT p.product_id, p.branch_id, p.warehouse_id, 'adjustment',
       p.old_qty, p.new_qty - p.old_qty, p.new_qty, 1
FROM tmp_stock_pick p
WHERE p.new_qty <> p.old_qty;

-- B3. Se elimina la fila duplicada bajo la sucursal sin nombre (ya fusionada arriba).
DELETE blank_ps
FROM ecom_product_stock blank_ps
JOIN tmp_wh_map m ON m.blank_wh_id = blank_ps.warehouse_id
JOIN ecom_product_stock real_ps ON real_ps.warehouse_id = m.real_wh_id
                               AND real_ps.product_id   = blank_ps.product_id
WHERE m.blank_wh_id IS NOT NULL;

-- B4. Lo que quedó bajo almacenes mapeados NO tenía contraparte real: se reasigna al
--     almacén real para no perder ese stock.
DROP TEMPORARY TABLE IF EXISTS tmp_repoint;
CREATE TEMPORARY TABLE tmp_repoint AS
SELECT ps.id AS stock_id, ps.product_id, ps.available_qty,
       m.real_branch_id, m.real_wh_id
FROM ecom_product_stock ps
JOIN tmp_wh_map m ON m.blank_wh_id = ps.warehouse_id;

UPDATE ecom_product_stock ps
JOIN tmp_repoint r ON r.stock_id = ps.id
SET ps.branch_id    = r.real_branch_id,
    ps.warehouse_id = r.real_wh_id
WHERE r.stock_id IS NOT NULL;

INSERT INTO ecom_stock_movements (product_id, branch_id, warehouse_id, movement_type,
                                  quantity_before, quantity_change, quantity_after, updated_by)
SELECT r.product_id, r.real_branch_id, r.real_wh_id, 'adjustment',
       0, r.available_qty, r.available_qty, 1
FROM tmp_repoint r
WHERE r.available_qty > 0;

-- B5. TODOS los movimientos de esas sucursales y almacenes: soft delete.
--     (Los 'adjustment' recién creados están en almacenes reales, no se tocan.)
UPDATE ecom_stock_movements sm
SET sm.deleted_at = NOW()
WHERE sm.deleted_at IS NULL
  AND (sm.branch_id    IN (SELECT t.id FROM tmp_blank_branch t)
    OR sm.warehouse_id IN (SELECT t.id FROM tmp_blank_wh t));

-- B6. Stock que sigue bajo sucursal/almacén sin nombre (almacenes SIN destino único):
--     borrado físico, ecom_product_stock no tiene deleted_at.
DELETE FROM ecom_product_stock
WHERE branch_id    IN (SELECT t.id FROM tmp_blank_branch t)
   OR warehouse_id IN (SELECT t.id FROM tmp_blank_wh t);

-- B7. Almacenes y sucursales sin nombre: soft delete.
UPDATE ecom_warehouses
SET deleted_at = NOW()
WHERE deleted_at IS NULL
  AND id IN (SELECT t.id FROM tmp_blank_wh t);

UPDATE ecom_branches
SET deleted_at = NOW()
WHERE deleted_at IS NULL
  AND id IN (SELECT t.id FROM tmp_blank_branch t);

-- B8. Resultado por almacén real fusionado (LEN/LEON debe verse con la cantidad correcta).
SELECT 'RESULTADO' AS reporte, b.name AS sucursal, w.name AS almacen,
       COUNT(ps.id) AS filas_stock, COALESCE(SUM(ps.available_qty), 0) AS unidades
FROM ecom_warehouses w
JOIN ecom_branches b ON b.id = w.branch_id
LEFT JOIN ecom_product_stock ps ON ps.warehouse_id = w.id
WHERE w.id IN (SELECT t.real_wh_id FROM tmp_wh_map t)
GROUP BY w.id, b.name, w.name;

-- B9. COMPUERTA: si queda algo sin nombre, esta inserción FALLA (error 3819/1048) y el
--     script se detiene ANTES del COMMIT. En ese caso ejecuta: ROLLBACK;
DROP TEMPORARY TABLE IF EXISTS tmp_gate;
CREATE TEMPORARY TABLE tmp_gate (ok TINYINT NOT NULL, CONSTRAINT chk_tmp_gate CHECK (ok = 1));
INSERT INTO tmp_gate (ok)
SELECT CASE WHEN
         (SELECT COUNT(*) FROM ecom_branches
           WHERE deleted_at IS NULL AND COALESCE(name, '') NOT REGEXP '[[:alnum:]]') = 0
     AND (SELECT COUNT(*) FROM ecom_warehouses w JOIN ecom_branches b ON b.id = w.branch_id
           WHERE w.deleted_at IS NULL AND COALESCE(b.name, '') NOT REGEXP '[[:alnum:]]') = 0
     AND (SELECT COUNT(*) FROM ecom_product_stock ps JOIN ecom_branches b ON b.id = ps.branch_id
           WHERE COALESCE(b.name, '') NOT REGEXP '[[:alnum:]]') = 0
     AND (SELECT COUNT(*) FROM ecom_stock_movements sm JOIN ecom_branches b ON b.id = sm.branch_id
           WHERE sm.deleted_at IS NULL AND COALESCE(b.name, '') NOT REGEXP '[[:alnum:]]') = 0
       THEN 1 ELSE 0 END;

COMMIT;

-- Para revertir lo ya confirmado, restaurar desde los respaldos:
--   bak_20261123_product_stock, bak_20261123_stock_movements,
--   bak_20261123_warehouses, bak_20261123_branches.


/* ===================================================================== */
/* PARTE C — BLINDAJE EN BASE DE DATOS (después del COMMIT de la B)      */
/* ===================================================================== */
-- Los ALTER hacen commit implícito, por eso van fuera de la transacción.
-- El CHECK valida las filas existentes: primero se renombran las sucursales sin nombre
-- que ya están dadas de baja. Si quedara una ACTIVA sin nombre, el ALTER falla a
-- propósito (la Parte B no quedó completa).

UPDATE ecom_branches
SET name = CONCAT('(sin nombre) #', id)
WHERE deleted_at IS NOT NULL
  AND COALESCE(name, '') NOT REGEXP '[[:alnum:]]';

ALTER TABLE ecom_branches
    ADD CONSTRAINT chk_ecom_branches_name_visible CHECK (name REGEXP '[[:alnum:]]');

-- Almacenes (opcional): revisa antes que no haya almacenes ACTIVOS con nombre vacío.
--   SELECT id, name FROM ecom_warehouses WHERE deleted_at IS NULL AND name NOT REGEXP '[[:alnum:]]';
-- UPDATE ecom_warehouses SET name = CONCAT('(sin nombre) #', id)
--  WHERE deleted_at IS NOT NULL AND COALESCE(name, '') NOT REGEXP '[[:alnum:]]';
-- ALTER TABLE ecom_warehouses
--     ADD CONSTRAINT chk_ecom_warehouses_name_visible CHECK (name REGEXP '[[:alnum:]]');

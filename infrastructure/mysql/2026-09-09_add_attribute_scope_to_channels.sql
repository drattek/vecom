-- ADR 0004 — Atributos personalizados en Odoo: modelo clave-valor por producto.
--
-- Nueva columna ecom_channels.attribute_scope:
--   category (default) → comportamiento actual: el checklist de atributos exige
--                        categoría asignada y mapeada al canal (Mercado Libre).
--   product            → los atributos son por producto, no por categoría; el
--                        checklist lista los slots aplicables sin exigir categoría
--                        (Odoo / website.sale.product.info).
--
-- Ejecutar a mano contra la base de core-orchestrator.

ALTER TABLE ecom_channels
    ADD COLUMN attribute_scope ENUM('category','product') NOT NULL DEFAULT 'category'
    AFTER description;

UPDATE ecom_channels
SET attribute_scope = 'product'
WHERE code = 'ODOO' AND deleted_at IS NULL;

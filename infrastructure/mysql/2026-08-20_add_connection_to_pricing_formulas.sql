-- Lets a pricing formula also depend on the channel connection, not just the
-- brand. Resolution priority (most to least specific), enforced in Go via
-- PricingFormulaRepository.Resolve, not here:
--   1. brand_id = X AND connection_id = Y   (exact match)
--   2. brand_id IS NULL AND connection_id = Y  (channel-wide, any brand)
--   3. brand_id = X AND connection_id IS NULL  (brand-wide, any channel — today's behavior)
--   4. brand_id IS NULL AND connection_id IS NULL  (universal default)
--
-- uq_pricing_formula_brand doubled as the supporting index for
-- fk_pricing_formula_brand, so it can't simply be dropped (MySQL error 1553,
-- "needed in a foreign key constraint") — idx_pricing_formula_brand takes
-- over that role. Uniqueness per (brand_id, connection_id) slot can't be a
-- DB-level unique key either: MySQL treats every NULL as distinct inside a
-- unique key, so a composite key would let duplicate (NULL, NULL) or
-- (X, NULL) rows through silently. It stays enforced in
-- PricingFormulaService, same as the single-NULL case does today.

ALTER TABLE ecom_pricing_formulas
    ADD COLUMN connection_id bigint unsigned DEFAULT NULL AFTER brand_id,
    ADD INDEX idx_pricing_formula_brand (brand_id),
    ADD CONSTRAINT fk_pricing_formula_connection
        FOREIGN KEY (connection_id) REFERENCES ecom_channel_connections(id),
    DROP KEY uq_pricing_formula_brand;

-- Adds a third matching dimension to pricing formulas: the price list that
-- won when resolving a product's effective price (ecom_product_prices via
-- ProductPricesRepository.FindEffectivePrice). Lets a formula be scoped to
-- a specific price list — e.g. an Odoo-only price list that needs a
-- different markup than the general catalog — on top of the existing
-- brand/connection dimensions. price_list_id NULL keeps acting as the
-- "any price list" wildcard, same as brand_id/connection_id already do.
--
-- Specificity order used by PricingFormulaRepository.Resolve, most to
-- least specific: connection match beats brand match beats price-list
-- match; ties broken by matching more dimensions overall. See that
-- method's doc comment for the full 8-slot table.

ALTER TABLE ecom_pricing_formulas
    ADD COLUMN price_list_id bigint unsigned DEFAULT NULL AFTER connection_id,
    ADD CONSTRAINT fk_pricing_formula_price_list
        FOREIGN KEY (price_list_id) REFERENCES ecom_price_list(id);

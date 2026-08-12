-- Adds MercadoLibre-style compatibility qualifiers (motor, position, side) to
-- product-vehicle compatibility. These describe how a specific product occupies
-- a vehicle fitment (e.g. same Sentra 2015-2018 fitment, but a left-side mirror
-- vs a right-side mirror are two different products/compatibilities), so they
-- live on the join table, not on ecom_vehicle_fitments (which stays a plain,
-- reusable brand/model/year description of the vehicle itself).
--
-- motor/position/side are NOT NULL DEFAULT '' rather than nullable: MySQL
-- treats each NULL as distinct inside a unique key, so nullable columns here
-- would let the "same combination" rule silently stop working for every
-- non-MercadoLibre import that doesn't set these fields. Empty string keeps
-- the uniqueness check meaningful whether or not the fields are used.

ALTER TABLE ecom_product_vehicle_compatibility
    ADD COLUMN motor    varchar(100) NOT NULL DEFAULT '' AFTER vehicle_fitment_id,
    ADD COLUMN position varchar(100) NOT NULL DEFAULT '' AFTER motor,
    ADD COLUMN side     varchar(100) NOT NULL DEFAULT '' AFTER position;

ALTER TABLE ecom_product_vehicle_compatibility
    DROP INDEX uq_product_vehicle_fitment,
    ADD UNIQUE KEY uq_product_vehicle_fitment (product_id, vehicle_fitment_id, motor, position, side);

-- Same fields on the pending/staging table, so a bulk import for a SKU that
-- doesn't have a product yet doesn't lose motor/position/side by the time
-- ProductService resolves it into a real compatibility row.

ALTER TABLE ecom_pending_product_vehicle_fitments
    ADD COLUMN motor    varchar(100) NOT NULL DEFAULT '' AFTER vehicle_fitment_id,
    ADD COLUMN position varchar(100) NOT NULL DEFAULT '' AFTER motor,
    ADD COLUMN side     varchar(100) NOT NULL DEFAULT '' AFTER position;

ALTER TABLE ecom_pending_product_vehicle_fitments
    DROP INDEX uq_pending_sku_fitment,
    ADD UNIQUE KEY uq_pending_sku_fitment (sku, vehicle_fitment_id, motor, position, side);

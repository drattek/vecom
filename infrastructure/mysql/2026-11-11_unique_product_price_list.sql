-- Una sola fila de precio por producto y lista.
--
-- Sin esta llave, una carga manual del 2-nov-2026 (INSERT ... ON DUPLICATE KEY
-- UPDATE, que sin llave única siempre inserta) duplicó 7,343 filas en
-- precios_maquinaria con el precio de venta de D365 en vez del costo promedio.
-- FindEffectivePrice toma la fila más reciente, así que 304 productos quedaron
-- con base 0 y miles con el margen aplicado sobre el precio de venta.
--
-- Aplicada en producción el 2026-11-11, después de borrar las filas de esa
-- carga. La lista precios_maquinaria guarda el costo promedio de D365
-- (dyn.ItemCost) y solo la escribe el sync diario (sync_erp_stock).
ALTER TABLE ecom_product_prices
    ADD UNIQUE KEY uq_product_price_list (product_id, price_list_id);

-- Recalcula ecom_product_dimensions.volume para las filas existentes.
--
-- A partir de ahora el volumen lo calcula el core en cada escritura
-- (ProductDimensionsRepository.Create/Update: volume = ROUND(length*width*height, 2),
-- en cm³ — el envío siempre es en caja rectangular, el diámetro no interviene).
-- Este script alinea los datos históricos, que hasta ahora se cargaban a mano
-- o quedaban en el default 1.00.
--
-- Idempotente: el WHERE deja fuera las filas que ya coinciden, así que solo
-- toca (y solo mueve updated_at de) las que realmente cambian. Se puede volver
-- a correr sin efecto.
--
-- El campo lo consume OdooProductSyncService.resolveDimensions (convierte cm³ -> m³).

UPDATE ecom_product_dimensions
SET volume = ROUND(length * width * height, 2)
WHERE deleted_at IS NULL
  AND volume <> ROUND(length * width * height, 2);

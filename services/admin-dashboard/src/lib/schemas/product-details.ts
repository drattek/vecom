import { z } from 'zod'

// Las secciones de la página de detalle de producto. Cada una corresponde a un
// endpoint /api/products/{id}/details/* del core-orchestrator, que ya devuelve
// las llaves foráneas resueltas a nombre/URL (marca, categoría, almacén,
// divisa, atributo, archivo) — el front no resuelve catálogos por su cuenta.

// --- General ---------------------------------------------------------------

export const productGeneralSchema = z.object({
  id: z.number(),
  sku: z.string(),
  partNumber: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  shortDescription: z.string().nullish(),
  productType: z.string(),
  status: z.string().nullish(),
  isSellable: z.boolean(),
  isStockable: z.boolean(),
  brandId: z.number().nullish(),
  brandName: z.string().nullish(),
  categoryId: z.number().nullish(),
  categoryName: z.string().nullish(),
  categoryPath: z.string().nullish(),
  sourceId: z.number(),
  sourceName: z.string().nullish(),
  coverImage: z.string().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
  createdBy: z.string().nullish(),
  updatedBy: z.string().nullish(),
})

// productGeneralPatchSchema valida la edición inline de la sección General. Es
// parcial: cada card envía solo las claves que edita (Identificación → name;
// Clasificación → brandId/categoryId; Descripción → description/shortDescription),
// reflejando el PATCH dinámico del core (PATCH /api/products/{id}/details/general).
// brandId/categoryId: 0 = "sin marca/categoría" (limpia la relación); > 0 = ese id.
export const productGeneralPatchSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, 'El nombre es obligatorio')
      .max(255, 'Máximo 255 caracteres'),
    shortDescription: z.string().trim().max(255, 'Máximo 255 caracteres'),
    description: z.string(),
    brandId: z.number().int().nonnegative(),
    categoryId: z.number().int().nonnegative(),
  })
  .partial()

export type ProductGeneralPatch = z.infer<typeof productGeneralPatchSchema>

// Opciones de marca para el selector (GET /api/brands/options).
export const brandOptionSchema = z.object({ id: z.number(), name: z.string() })
export const brandOptionsResponseSchema = z.object({ brands: z.array(brandOptionSchema) })
export type BrandOption = z.infer<typeof brandOptionSchema>

// --- Multimedia ------------------------------------------------------------

export const productMediaItemSchema = z.object({
  id: z.number(),
  fileId: z.number(),
  url: z.string().nullish(),
  filename: z.string(),
  mimeType: z.string(),
  extension: z.string(),
  size: z.number(),
  width: z.number().nullish(),
  height: z.number().nullish(),
  isFirst: z.boolean().optional().default(false),
  type: z.string().optional().default(''),
  createdAt: z.string(),
})

export const productMediaSectionSchema = z.object({
  images: z.array(productMediaItemSchema),
  videos: z.array(productMediaItemSchema),
  attachments: z.array(productMediaItemSchema),
})

// Alta masiva de imágenes por URL para un producto (Multimedia → "Agregar
// imágenes"). El core valida cada URL con un HEAD y reporta el resultado por
// item, así que un item roto no tumba el resto.
export const productImageImportResultSchema = z.object({
  url: z.string(),
  success: z.boolean(),
  isFirst: z.boolean().optional().default(false),
  fileId: z.number().nullish(),
  productImageId: z.number().nullish(),
  error: z.string().nullish(),
})

export const productImageImportResponseSchema = z.object({
  results: z.array(productImageImportResultSchema),
})

export type ProductImageImportResult = z.infer<typeof productImageImportResultSchema>
export type ProductImageImportResponse = z.infer<typeof productImageImportResponseSchema>

// Alta masiva de archivos adjuntos por URL (Multimedia → "Archivos adjuntos").
// Cada archivo lleva su tipo (manual / datasheet / certificate / image); no hay
// concepto de "principal".
export const productAttachmentImportResultSchema = z.object({
  url: z.string(),
  type: z.string().optional().default(''),
  success: z.boolean(),
  fileId: z.number().nullish(),
  error: z.string().nullish(),
})

export const productAttachmentImportResponseSchema = z.object({
  results: z.array(productAttachmentImportResultSchema),
})

export type ProductAttachmentImportResult = z.infer<typeof productAttachmentImportResultSchema>

// Alta masiva de videos por URL (Multimedia → "Videos"). El caso más simple:
// solo el disco y las URLs — sin orden, portada ni tipo.
export const productVideoImportResultSchema = z.object({
  url: z.string(),
  success: z.boolean(),
  fileId: z.number().nullish(),
  videoId: z.number().nullish(),
  error: z.string().nullish(),
})

export const productVideoImportResponseSchema = z.object({
  results: z.array(productVideoImportResultSchema),
})

export type ProductVideoImportResult = z.infer<typeof productVideoImportResultSchema>

// --- Precios ---------------------------------------------------------------

export const productPriceSchema = z.object({
  id: z.number(),
  priceListId: z.number(),
  priceListName: z.string(),
  priceListPriority: z.number(),
  status: z.string(),
  validFrom: z.string(),
  validTo: z.string(),
  price: z.string(),
  currencyId: z.number(),
  currencyCode: z.string(),
  currencySymbol: z.string(),
  margin: z.string(),
  taxIncluded: z.boolean(),
  updatedAt: z.string(),
  updatedBy: z.string().nullish(),
  isEffective: z.boolean().optional().default(false),
})

export const productPriceHistorySchema = z.object({
  id: z.number(),
  priceListId: z.number(),
  priceListName: z.string().nullish(),
  oldPrice: z.string(),
  newPrice: z.string(),
  currencyCode: z.string().nullish(),
  currencySymbol: z.string().nullish(),
  updatedBy: z.string().nullish(),
  createdAt: z.string(),
})

// El historial de cambios de precio se pagina aparte
// (GET .../details/pricing/history): en un producto viejo son miles de filas.
export const productPricingSectionSchema = z.object({
  current: z.array(productPriceSchema),
})

export const paginatedProductPriceHistorySchema = z.object({
  data: z.array(productPriceHistorySchema),
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
})

// --- Inventario ------------------------------------------------------------

export const productStockLocationSchema = z.object({
  id: z.number(),
  branchId: z.number(),
  branchName: z.string(),
  warehouseId: z.number(),
  warehouseName: z.string(),
  availableQty: z.number(),
  lastSyncAt: z.string().nullish(),
})

export const productStockMovementSchema = z.object({
  id: z.number(),
  branchName: z.string().nullish(),
  warehouseName: z.string().nullish(),
  movementType: z.string(),
  quantityBefore: z.number(),
  quantityChange: z.number(),
  quantityAfter: z.number(),
  updatedBy: z.string().nullish(),
  createdAt: z.string(),
})

// El historial de movimientos se pagina aparte
// (GET .../details/inventory/movements): un producto sincronizado acumula una
// fila por cada sync del ERP que cambió su cantidad.
export const productInventorySectionSchema = z.object({
  totalStock: z.number(),
  byLocation: z.array(productStockLocationSchema),
})

export const paginatedProductStockMovementsSchema = z.object({
  data: z.array(productStockMovementSchema),
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
})

// --- Números de parte -------------------------------------------------------

export const productAlternatePartNumberSchema = z.object({
  partNumber: z.string(),
  type: z.string(),
  brandId: z.number(),
  brandName: z.string().nullish(),
  createdAt: z.string(),
})

export const productSupersessionSchema = z.object({
  id: z.number(),
  direction: z.enum(['supersedes', 'superseded_by']),
  oldPartNumber: z.string(),
  newPartNumber: z.string(),
  otherPartNumber: z.string(),
  otherProductId: z.number().nullish(),
  otherProductSku: z.string().nullish(),
  resolved: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string().nullish(),
})

export const productPartNumbersSectionSchema = z.object({
  partNumber: z.string(),
  alternates: z.array(productAlternatePartNumberSchema),
  supersessions: z.array(productSupersessionSchema),
})

// --- Atributos ---------------------------------------------------------------

export const productDimensionsSchema = z.object({
  weight: z.string(),
  length: z.string(),
  width: z.string(),
  height: z.string(),
  diameter: z.string(),
  volume: z.string(),
})

// productDimensionsPatchSchema valida la edición inline de la sección
// Dimensiones (PATCH /api/products/{id}/details/attributes/dimensions). Los
// valores viajan como texto — el core los normaliza a decimal(10,2). El volumen
// no se envía: lo calcula el core como largo*ancho*alto (cm³).
const dimensionValue = z
  .string()
  .trim()
  .min(1, 'Requerido')
  .refine((value) => Number.isFinite(Number(value)), 'Número inválido')
  .refine((value) => Number(value) >= 0, 'Debe ser mayor o igual a 0')
  .refine((value) => Number(value) <= 99_999_999.99, 'Valor demasiado grande')

export const productDimensionsPatchSchema = z.object({
  weight: dimensionValue,
  length: dimensionValue,
  width: dimensionValue,
  height: dimensionValue,
  diameter: dimensionValue,
})

export type ProductDimensionsPatch = z.infer<typeof productDimensionsPatchSchema>

export const productSeoSchema = z.object({
  metaTitle: z.string().nullish(),
  metaDescription: z.string().nullish(),
  keywords: z.string().nullish(),
})

// productSeoPatchSchema valida la edición inline de la sección SEO
// (PATCH /api/products/{id}/details/attributes/seo). Todos los campos son
// opcionales: vacío limpia ese metadato (se guarda NULL).
export const productSeoPatchSchema = z.object({
  metaTitle: z.string().trim().max(255, 'Máximo 255 caracteres'),
  metaDescription: z.string().trim().max(255, 'Máximo 255 caracteres'),
  keywords: z.string().trim().max(5000, 'Máximo 5000 caracteres'),
})

export type ProductSeoPatch = z.infer<typeof productSeoPatchSchema>

export const productAttributeValueSchema = z.object({
  id: z.number(),
  attributeId: z.number(),
  code: z.string(),
  name: z.string(),
  dataType: z.string(),
  unit: z.string().nullish(),
  value: z.string(),
  optionId: z.number().nullish(),
  updatedBy: z.string().nullish(),
})

export const productAttributesSectionSchema = z.object({
  dimensions: productDimensionsSchema.nullish(),
  seo: productSeoSchema.nullish(),
  attributes: z.array(productAttributeValueSchema),
})

// --- Checklist de atributos por canal ---------------------------------------
// GET /api/products/{id}/details/attributes/checklist?connectionId=N — qué
// atributos espera el canal para la categoría del producto, requerido/opcional
// y su valor actual.

export const attributeChecklistOptionSchema = z.object({ id: z.number(), value: z.string() })

export const attributeChecklistItemSchema = z.object({
  attributeId: z.number(),
  code: z.string(),
  name: z.string(),
  dataType: z.string(),
  unit: z.string().nullish(),
  externalLabel: z.string().nullish(),
  isRequired: z.boolean(),
  value: z.string(),
  optionId: z.number().nullish(),
  productAttributeId: z.number().nullish(),
  options: z.array(attributeChecklistOptionSchema).optional().default([]),
})

export const attributeChecklistSchema = z.object({
  connectionId: z.number(),
  channelName: z.string(),
  connectionName: z.string(),
  // 'category' (Mercado Libre): atributos por categoría. 'product' (Odoo):
  // atributos por producto — el front ofrece un alta libre de atributos.
  attributeScope: z.enum(['category', 'product']),
  categoryId: z.number().nullish(),
  categoryName: z.string().nullish(),
  // 'needs_selection': el canal es por categoría y todavía no hay categoría
  // externa resuelta para esta conexión — ni por selección manual (ver ADR
  // 0005) ni por el mapeo legado de la categoría del producto. El front debe
  // mostrar el selector de categoría en vez de la lista de atributos.
  state: z.enum(['needs_selection', 'ok']),
  externalCategoryId: z.string().nullish(),
  externalCategoryName: z.string().nullish(),
  requiredMissing: z.number(),
  items: z.array(attributeChecklistItemSchema),
  autoCovered: z.array(z.object({ label: z.string(), systemField: z.string() })),
})

export type AttributeChecklist = z.infer<typeof attributeChecklistSchema>
export type AttributeChecklistItem = z.infer<typeof attributeChecklistItemSchema>

// --- Sincronización ----------------------------------------------------------
// GET /api/products/{id}/details/sync — una caja por conexión activa con lo
// que el producto ya tiene sincronizado ahí (id externo, categoría externa,
// título, estado, última sincronización); en conexiones con
// allowsMultipleListings además la compatibilidad de cada publicación y las
// compatibilidades que aún no tienen publicación (pending).

export const syncCompatibilitySchema = z.object({
  vehicleFitmentId: z.number(),
  brandName: z.string(),
  model: z.string(),
  yearStart: z.number(),
  yearEnd: z.number().nullish(),
  motor: z.string().optional().default(''),
  position: z.string().optional().default(''),
  side: z.string().optional().default(''),
})

export const syncListingSchema = z.object({
  id: z.number(),
  listingTitle: z.string().nullish(),
  externalId: z.string().nullish(),
  externalCategoryId: z.string().nullish(),
  externalCategoryName: z.string().nullish(),
  status: z.string(),
  isEnabled: z.boolean(),
  lastSyncedAt: z.string().nullish(),
  compatibility: syncCompatibilitySchema.nullish(),
})

export const syncConnectionSchema = z.object({
  connectionId: z.number(),
  channelName: z.string(),
  connectionName: z.string(),
  environment: z.string(),
  allowsMultipleListings: z.boolean(),
  listings: z.array(syncListingSchema),
  pending: z.array(syncCompatibilitySchema),
})

export const productSyncSectionSchema = z.object({
  connections: z.array(syncConnectionSchema),
})

export type SyncCompatibility = z.infer<typeof syncCompatibilitySchema>
export type SyncListing = z.infer<typeof syncListingSchema>
export type SyncConnection = z.infer<typeof syncConnectionSchema>
export type ProductSyncSection = z.infer<typeof productSyncSectionSchema>

// Sugerencia de categoría de MercadoLibre por texto —
// GET /api/marketplaces/mercadolibre/category-predictor?connectionId=&title=&limit=1.
// category_id/category_name vienen tal cual los devuelve la API de MercadoLibre
// (snake_case), a diferencia del resto de las respuestas del core.
export const categoryPredictionSchema = z.object({
  category_id: z.string(),
  category_name: z.string(),
})

export const categoryPredictionsResponseSchema = z.object({
  predictions: z.array(categoryPredictionSchema),
})

export type CategoryPrediction = z.infer<typeof categoryPredictionSchema>

// Selección manual de categoría externa por conexión, antes de publicar (ver
// ADR 0005) — POST .../details/sync/{connectionId}/category con
// { externalCategoryId }.
export const channelProductCategorySelectionSchema = z.object({
  productId: z.number(),
  connectionId: z.number(),
  categoryId: z.number(),
  externalCategoryId: z.string(),
  externalCategoryName: z.string().optional().default(''),
})

export type ChannelProductCategorySelection = z.infer<typeof channelProductCategorySelectionSchema>

// --- Compatibilidades ------------------------------------------------------
// GET /api/products/{id}/details/compatibilities — solo lectura: las
// compatibilidades de vehículo del producto, con el fitment ya resuelto a
// marca/modelo/años y los calificadores motor/posición/lado de la tabla de
// enlace.

export const productVehicleCompatibilitySchema = z.object({
  id: z.number(),
  vehicleFitmentId: z.number(),
  brandName: z.string().nullish(),
  model: z.string(),
  yearStart: z.number(),
  yearEnd: z.number().nullish(),
  motor: z.string().nullish(),
  position: z.string().nullish(),
  side: z.string().nullish(),
  createdAt: z.string(),
})

export const productCompatibilitiesSectionSchema = z.object({
  vehicles: z.array(productVehicleCompatibilitySchema),
})

export type ProductVehicleCompatibility = z.infer<typeof productVehicleCompatibilitySchema>
export type ProductCompatibilitiesSection = z.infer<typeof productCompatibilitiesSectionSchema>

// Publicar el producto en una conexión sin sincronización todavía (botón
// "Publicar" de la vista Sincronización) — POST /api/channel-listings/publish
// con { connectionId, skus: [sku] }. Puede devolver más de un resultado si la
// conexión permite múltiples publicaciones (una por compatibilidad).
export const listingOutcomeSchema = z.object({
  sku: z.string(),
  vehicleFitmentId: z.number().nullish(),
  title: z.string().optional().default(''),
  success: z.boolean(),
  skipped: z.boolean().optional().default(false),
  externalId: z.string().optional().default(''),
  missingRequiredAttributes: z.array(z.string()).optional().default([]),
  missingOptionalAttributes: z.array(z.string()).optional().default([]),
  error: z.string().optional().default(''),
})

export const createListingsResultSchema = z.object({
  results: z.array(listingOutcomeSchema),
})

export type ListingOutcome = z.infer<typeof listingOutcomeSchema>
export type CreateListingsResult = z.infer<typeof createListingsResultSchema>

// Resincronizar el producto en una conexión (botón "Resincronizar" de la vista
// Sincronización) — POST /api/channel-listings/resync con
// { connectionId, productId }. A diferencia de "Publicar", empuja todos los
// campos que el marketplace permite modificar en una publicación ya creada
// (no solo precio/stock), a cada publicación activa del producto en esa
// conexión.
export const refreshOutcomeSchema = z.object({
  productId: z.number(),
  sku: z.string().optional().default(''),
  externalId: z.string().optional().default(''),
  updated: z.boolean().optional().default(false),
  skipped: z.boolean().optional().default(false),
  error: z.string().optional().default(''),
})

export const resyncListingsResultSchema = z.object({
  results: z.array(refreshOutcomeSchema),
})

export type RefreshOutcome = z.infer<typeof refreshOutcomeSchema>
export type ResyncListingsResult = z.infer<typeof resyncListingsResultSchema>

export type ProductGeneral = z.infer<typeof productGeneralSchema>
export type ProductDimensions = z.infer<typeof productDimensionsSchema>
export type ProductSeo = z.infer<typeof productSeoSchema>
export type ProductMediaItem = z.infer<typeof productMediaItemSchema>
export type ProductMediaSection = z.infer<typeof productMediaSectionSchema>
export type ProductPrice = z.infer<typeof productPriceSchema>
export type ProductPriceHistory = z.infer<typeof productPriceHistorySchema>
export type ProductStockLocation = z.infer<typeof productStockLocationSchema>
export type ProductStockMovement = z.infer<typeof productStockMovementSchema>
export type PaginatedProductStockMovements = z.infer<typeof paginatedProductStockMovementsSchema>
export type PaginatedProductPriceHistory = z.infer<typeof paginatedProductPriceHistorySchema>
export type ProductAlternatePartNumber = z.infer<typeof productAlternatePartNumberSchema>
export type ProductSupersession = z.infer<typeof productSupersessionSchema>
export type ProductAttributeValue = z.infer<typeof productAttributeValueSchema>

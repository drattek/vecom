import { z } from 'zod'

export type CategoryTree = {
  id: number
  name: string
  parentId?: number | null
  createdAt: string
  updatedAt: string
  children: CategoryTree[]
}

export const categoryTreeSchema: z.ZodType<CategoryTree> = z.lazy(() =>
  z.object({
    id: z.number(),
    name: z.string(),
    parentId: z.number().nullish(),
    createdAt: z.string(),
    updatedAt: z.string(),
    children: z.array(categoryTreeSchema).default([]),
  })
)

export const categoriesResponseSchema = z.object({
  categories: z.array(categoryTreeSchema),
})

export type CategoriesResponse = z.infer<typeof categoriesResponseSchema>

// Respuesta de GET /api/categories/{id} — la categoría suelta, sin el árbol.
export const categoryDetailSchema = z.object({
  id: z.number(),
  name: z.string(),
  parentId: z.number().nullish(),
  createdBy: z.number(),
  updatedBy: z.number().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export type CategoryDetail = z.infer<typeof categoryDetailSchema>

// Respuesta de GET /api/categories/{id}/channel-mappings — una fila por
// conexión de canal activa, con el mapeo a categoría externa si existe.
export const categoryChannelMappingSchema = z.object({
  connectionId: z.number(),
  connectionName: z.string(),
  channelId: z.number(),
  channelName: z.string(),
  channelCode: z.string(),
  environment: z.string(),
  externalCategoryId: z.string().nullish(),
  externalCategoryName: z.string().nullish(),
  mappedAt: z.string().nullish(),
})

export type CategoryChannelMapping = z.infer<typeof categoryChannelMappingSchema>

export const categoryChannelMappingsResponseSchema = z.object({
  mappings: z.array(categoryChannelMappingSchema),
})

// --- Importar categoría desde un canal ---------------------------------

// GET /api/category-import/sources — canales con ≥1 conexión activa.
export const categoryImportSourceSchema = z.object({
  channelId: z.number(),
  channelCode: z.string(),
  channelName: z.string(),
  supported: z.boolean(),
  connections: z.array(
    z.object({
      id: z.number(),
      name: z.string(),
      environment: z.string(),
    }),
  ),
})

export type CategoryImportSource = z.infer<typeof categoryImportSourceSchema>

export const categoryImportSourcesResponseSchema = z.object({
  sources: z.array(categoryImportSourceSchema),
})

// GET /api/category-import/tree — un nivel del árbol de categorías externas.
// `leaf` null ⇒ el canal no lo sabe sin expandir (MercadoLibre).
export const categoryImportNodeSchema = z.object({
  externalId: z.string(),
  name: z.string(),
  leaf: z.boolean().nullish(),
})

export type CategoryImportNode = z.infer<typeof categoryImportNodeSchema>

export const categoryImportTreeResponseSchema = z.object({
  nodes: z.array(categoryImportNodeSchema),
})

// POST /api/category-import — resultado de importar una hoja.
export const categoryImportResultSchema = z.object({
  categoryId: z.number(),
  name: z.string(),
  createdCount: z.number(),
  alreadyLinked: z.boolean(),
  mappedConnectionIds: z.array(z.number()),
})

export type CategoryImportResult = z.infer<typeof categoryImportResultSchema>

// POST /api/category-import/mapping — vincular/reemplazar el mapeo de una
// categoría local con una hoja externa en una conexión.
export const setCategoryMappingResultSchema = z.object({
  categoryId: z.number(),
  connectionId: z.number(),
  externalCategoryId: z.string(),
  externalCategoryName: z.string(),
  replaced: z.boolean(),
})

export type SetCategoryMappingResult = z.infer<typeof setCategoryMappingResultSchema>

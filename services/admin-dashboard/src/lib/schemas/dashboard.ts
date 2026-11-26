import { z } from 'zod'

export type AttentionState = 'error' | 'not_published' | 'under_review' | 'paused_manual' | 'paused_zero_stock'

export type CatalogFilter =
  | 'no_image'
  | 'no_price'
  | 'no_brand'
  | 'no_category'
  | 'no_stock'
  | 'category_unmapped'
  | 'ready_unpublished'

export const attentionRowSchema = z.object({
  kind: z.enum(['listing', 'not_published']),
  id: z.number(),
  productId: z.number(),
  sku: z.string(),
  productName: z.string(),
  connectionName: z.string(),
  vehicleFitmentId: z.number().nullish(),
  listingTitle: z.string().nullish(),
  externalId: z.string().nullish(),
  lastError: z.string().nullish(),
  lastActivityAt: z.string().nullish(),
})

export const attentionPageSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  rows: z.array(attentionRowSchema),
})

export const catalogCountSchema = z.object({ total: z.number() })

export const catalogRowSchema = z.object({
  productId: z.number(),
  sku: z.string(),
  productName: z.string(),
  brandName: z.string().nullish(),
  categoryName: z.string().nullish(),
  hasImage: z.boolean(),
  hasPrice: z.boolean(),
  hasBrand: z.boolean(),
  hasCategory: z.boolean(),
  hasStock: z.boolean(),
})

export const catalogPageSchema = z.object({
  offset: z.number(),
  pageSize: z.number(),
  rows: z.array(catalogRowSchema),
})

export const connectionHealthSchema = z.object({
  connectionId: z.number(),
  connectionName: z.string(),
  channelName: z.string(),
  channelCode: z.string(),
  status: z.string(),
  environment: z.string(),
  hasStatus: z.boolean(),
  authenticated: z.boolean(),
  lastAuth: z.string().nullish(),
  expiresAt: z.string().nullish(),
  lastError: z.string().nullish(),
})

export const connectionsHealthSchema = z.object({ connections: z.array(connectionHealthSchema) })

export const inventoryHealthSchema = z.object({
  sources: z.array(
    z.object({
      sourceId: z.number(),
      sourceName: z.string(),
      stockRows: z.number(),
      lastChangeAt: z.string().nullish(),
    }),
  ),
})

export const queueHealthSchema = z.object({
  queues: z.array(
    z.object({
      connectionId: z.number(),
      connectionName: z.string(),
      syncType: z.string(),
      pending: z.number(),
      processing: z.number(),
      failed: z.number(),
      oldestPendingAt: z.string().nullish(),
    }),
  ),
})

export type AttentionRow = z.infer<typeof attentionRowSchema>
export type CatalogRow = z.infer<typeof catalogRowSchema>
export type ConnectionHealth = z.infer<typeof connectionHealthSchema>

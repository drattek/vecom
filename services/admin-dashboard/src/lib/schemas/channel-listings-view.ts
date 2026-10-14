import { z } from 'zod'

export const listingStateSchema = z.enum([
  'published',
  'paused_zero_stock',
  'paused_manual',
  'under_review',
  'closed',
  'pending',
  'error',
  'not_published',
])

export type ListingState = z.infer<typeof listingStateSchema>

export const listingViewRowSchema = z.object({
  kind: z.enum(['listing', 'not_published']),
  id: z.number(),
  productId: z.number(),
  sku: z.string(),
  productName: z.string(),
  connectionId: z.number(),
  connectionName: z.string(),
  vehicleFitmentId: z.number().nullish(),
  listingTitle: z.string().nullish(),
  externalId: z.string().nullish(),
  externalCategoryId: z.string().nullish(),
  rawStatus: z.string().nullish(),
  pausedReason: z.string().nullish(),
  state: listingStateSchema,
  lastError: z.string().nullish(),
  lastActivityAt: z.string().nullish(),
  attempts: z.number(),
})

export const paginatedListingViewSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  listings: z.array(listingViewRowSchema),
  stateCounts: z.record(z.string(), z.number()),
  withErrorCount: z.number(),
})

export const listingEventSchema = z.object({
  id: z.number(),
  productId: z.number(),
  connectionId: z.number(),
  vehicleFitmentId: z.number().nullish(),
  channelProductMapId: z.number().nullish(),
  externalId: z.string().nullish(),
  action: z.enum(['publish', 'refresh', 'resync', 'pause', 'reactivate', 'status_sync']),
  outcome: z.enum(['success', 'error']),
  statusBefore: z.string().nullish(),
  statusAfter: z.string().nullish(),
  message: z.string().nullish(),
  createdAt: z.string(),
})

export const listingHistoryResponseSchema = z.object({
  events: z.array(listingEventSchema),
})

export type ListingViewRow = z.infer<typeof listingViewRowSchema>
export type PaginatedListingView = z.infer<typeof paginatedListingViewSchema>
export type ListingEvent = z.infer<typeof listingEventSchema>

import { z } from 'zod'

import { listingStateSchema } from './channel-listings-view'

export const publicationReportConnectionSchema = z.object({
  connectionId: z.number(),
  connectionName: z.string(),
  state: listingStateSchema,
  listings: z.number(),
})

export const publicationReportRowSchema = z.object({
  productId: z.number(),
  sku: z.string(),
  name: z.string(),
  partNumber: z.string(),
  brandName: z.string().nullish(),
  sourceId: z.number(),
  sourceName: z.string(),
  imageCount: z.number(),
  hasMktImages: z.boolean(),
  stock: z.number(),
  price: z
    .object({
      amount: z.string(),
      currencyCode: z.string(),
      currencySymbol: z.string(),
    })
    .nullish(),
  odoo: z.array(publicationReportConnectionSchema),
  mercadoLibre: z.array(publicationReportConnectionSchema),
})

export const paginatedPublicationsReportSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  rows: z.array(publicationReportRowSchema),
})

export const sourcesResponseSchema = z.object({
  sources: z.array(z.object({ id: z.number(), code: z.string(), name: z.string() })),
})

export type PublicationReportConnection = z.infer<typeof publicationReportConnectionSchema>
export type PublicationReportRow = z.infer<typeof publicationReportRowSchema>

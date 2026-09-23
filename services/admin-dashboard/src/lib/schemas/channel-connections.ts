import { z } from 'zod'

export const channelConnectionSchema = z.object({
  id: z.number(),
  channelId: z.number(),
  channelName: z.string().nullish(),
  // ecom_channels.code (p.ej. "MERCADOLIBRE", "ODOO") — distingue si el
  // selector de categoría del checklist de atributos debe mostrar primero la
  // sugerencia del predictor (MercadoLibre) o el árbol directo. Ver ADR 0005.
  channelCode: z.string().optional().default(''),
  name: z.string(),
  status: z.string(),
  environment: z.string(),
  currencyId: z.number(),
  allowsMultipleListings: z.boolean(),
  createdBy: z.number(),
  updatedBy: z.number().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
  deletedAt: z.string().nullish(),
})

export const paginatedChannelConnectionsResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  channelConnections: z.array(channelConnectionSchema),
})

export type ChannelConnection = z.infer<typeof channelConnectionSchema>
export type PaginatedChannelConnectionsResponse = z.infer<typeof paginatedChannelConnectionsResponseSchema>

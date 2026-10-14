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
  // Si está en true, los productos con foto + precio + datos completos y
  // stock = 0 se publican/mantienen como suspendidos en este canal
  // (status "paused" en MercadoLibre, sale_ok=false en Odoo) en vez de
  // esperar a tener existencias. Cuando el stock regresa se reactivan
  // automáticamente — pero solo si el middleware fue quien los pausó
  // (paused_reason='zero_stock' en ecom_channel_product_map).
  syncZeroStockAsPaused: z.boolean().default(false),
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

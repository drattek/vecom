import { z } from 'zod'

export const priceListSchema = z.object({
  id: z.number(),
  name: z.string(),
  currency: z.number(),
  priority: z.number(),
  status: z.string(),
  validFrom: z.string(),
  validTo: z.string(),
  createdBy: z.number(),
  updatedBy: z.number().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedPriceListsResponseSchema = z.object({
  data: z.array(priceListSchema).nullable().transform((value) => value ?? []),
  total: z.number(),
})

export type PriceList = z.infer<typeof priceListSchema>
export type PaginatedPriceListsResponse = z.infer<typeof paginatedPriceListsResponseSchema>

import { z } from 'zod'

export const currencySchema = z.object({
  id: z.number(),
  name: z.string(),
  code: z.string(),
  symbol: z.string(),
  decimalPlaces: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedCurrenciesResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  currencies: z.array(currencySchema),
})

export type Currency = z.infer<typeof currencySchema>
export type PaginatedCurrenciesResponse = z.infer<typeof paginatedCurrenciesResponseSchema>

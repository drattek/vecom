import { z } from 'zod'

export const exchangeRateSchema = z.object({
  id: z.number(),
  fromCurrencyId: z.number(),
  fromCurrencyCode: z.string().optional(),
  fromCurrencySymbol: z.string().optional(),
  toCurrencyId: z.number(),
  toCurrencyCode: z.string().optional(),
  toCurrencySymbol: z.string().optional(),
  rate: z.string(),
  updatedBy: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedExchangeRatesResponseSchema = z.object({
  data: z.array(exchangeRateSchema),
  total: z.number(),
})

export type ExchangeRate = z.infer<typeof exchangeRateSchema>
export type PaginatedExchangeRatesResponse = z.infer<typeof paginatedExchangeRatesResponseSchema>

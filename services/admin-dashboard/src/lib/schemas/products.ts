import { z } from 'zod'

export const productPriceSchema = z.object({
  amount: z.string(),
  currencyCode: z.string(),
  currencySymbol: z.string(),
})

export const productChannelSchema = z.object({
  connectionId: z.number(),
  code: z.string(),
  name: z.string(),
})

export const productSchema = z.object({
  id: z.number(),
  sku: z.string(),
  partNumber: z.string(),
  name: z.string(),
  imageUrl: z.string().nullish(),
  brandName: z.string().nullish(),
  totalStock: z.number(),
  price: productPriceSchema.nullish(),
  channels: z.array(productChannelSchema).default([]),
})

export const paginatedProductsResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  products: z.array(productSchema),
})

export type Product = z.infer<typeof productSchema>
export type ProductPrice = z.infer<typeof productPriceSchema>
export type ProductChannel = z.infer<typeof productChannelSchema>
export type PaginatedProductsResponse = z.infer<typeof paginatedProductsResponseSchema>

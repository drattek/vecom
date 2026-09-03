import { z } from 'zod'

export const brandSchema = z.object({
  id: z.number(),
  name: z.string(),
  createdBy: z.number(),
  updatedBy: z.number().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
  deletedAt: z.string().nullish(),
})

export const paginatedBrandsResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  brands: z.array(brandSchema),
})

export type Brand = z.infer<typeof brandSchema>
export type PaginatedBrandsResponse = z.infer<typeof paginatedBrandsResponseSchema>

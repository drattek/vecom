import { z } from 'zod'

export type CategoryTree = {
  id: number
  name: string
  parentId?: number | null
  createdAt: string
  updatedAt: string
  children: CategoryTree[]
}

export const categoryTreeSchema: z.ZodType<CategoryTree> = z.lazy(() =>
  z.object({
    id: z.number(),
    name: z.string(),
    parentId: z.number().nullish(),
    createdAt: z.string(),
    updatedAt: z.string(),
    children: z.array(categoryTreeSchema).default([]),
  })
)

export const categoriesResponseSchema = z.object({
  categories: z.array(categoryTreeSchema),
})

export type CategoriesResponse = z.infer<typeof categoriesResponseSchema>

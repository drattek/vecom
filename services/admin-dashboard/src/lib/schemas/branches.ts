import { z } from 'zod'

export const branchSchema = z.object({
  id: z.number(),
  name: z.string(),
  createdBy: z.number(),
  updatedBy: z.number().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
  deletedAt: z.string().nullish(),
})

export const paginatedBranchesResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  branches: z.array(branchSchema),
})

export type Branch = z.infer<typeof branchSchema>
export type PaginatedBranchesResponse = z.infer<typeof paginatedBranchesResponseSchema>

import { z } from 'zod'

export const userSchema = z.object({
  id: z.number(),
  username: z.string(),
  role: z.string(),
  isActive: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedUsersResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  users: z.array(userSchema),
})

export type User = z.infer<typeof userSchema>
export type PaginatedUsersResponse = z.infer<typeof paginatedUsersResponseSchema>

import { z } from 'zod'

export const storageDiskSchema = z.object({
  id: z.number(),
  name: z.string(),
  code: z.string(),
  baseUrl: z.string(),
  bucket: z.string().optional(),
  endpoint: z.string().optional(),
  isPublic: z.boolean(),
})

export const paginatedStorageDisksResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  storageDisks: z.array(storageDiskSchema),
})

export type StorageDisk = z.infer<typeof storageDiskSchema>
export type PaginatedStorageDisksResponse = z.infer<typeof paginatedStorageDisksResponseSchema>
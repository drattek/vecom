import { z } from 'zod'

export const fileSchema = z.object({
  id: z.number(),
  diskId: z.number(),
  path: z.string(),
  filename: z.string(),
  originalFilename: z.string().optional(),
  mimeType: z.string(),
  fileType: z.string(),
  extension: z.string(),
  size: z.number(),
  checksum: z.string().optional(),
  width: z.number().optional(),
  height: z.number().optional(),
  isPublic: z.boolean(),
})

export const paginatedFilesResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  files: z.array(fileSchema),
})

export type File = z.infer<typeof fileSchema>
export type PaginatedFilesResponse = z.infer<typeof paginatedFilesResponseSchema>
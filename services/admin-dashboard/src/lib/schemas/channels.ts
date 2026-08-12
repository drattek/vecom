import { z } from 'zod'

export const channelSchema = z.object({
  id: z.number(),
  name: z.string(),
  code: z.string(),
  status: z.enum(['active', 'discontinued', 'hidden']),
  iconId: z.number().optional(),
  description: z.string().optional(),
  connectionCount: z.number(),
})

export const paginatedChannelsResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  channels: z.array(channelSchema),
})

export type Channel = z.infer<typeof channelSchema>
export type PaginatedChannelsResponse = z.infer<typeof paginatedChannelsResponseSchema>

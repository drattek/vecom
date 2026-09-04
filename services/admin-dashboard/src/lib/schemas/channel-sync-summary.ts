import { z } from 'zod'

export const channelSyncBrandSchema = z.object({
  id: z.number(),
  name: z.string(),
})

export const channelSyncRecentAddedSchema = z.object({
  date: z.string(),
  count: z.number(),
})

export const channelSyncSummarySchema = z.object({
  total: z.number(),
  withoutIssues: z.number(),
  underReview: z.number(),
  brands: z.array(channelSyncBrandSchema),
  recentlyAdded: z.array(channelSyncRecentAddedSchema),
})

export type ChannelSyncSummary = z.infer<typeof channelSyncSummarySchema>
export type ChannelSyncBrand = z.infer<typeof channelSyncBrandSchema>
export type ChannelSyncRecentAdded = z.infer<typeof channelSyncRecentAddedSchema>

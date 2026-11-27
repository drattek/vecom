import { z } from "zod"

export const notificationSettingsSchema = z.object({
    hasToken: z.boolean(),
    botUsername: z.string(),
    isEnabled: z.boolean(),
    dailySummaryTime: z.string(),
    timezone: z.string(),
    erpSilenceHours: z.number(),
    connectionCheckMinutes: z.number(),
})

export const myNotificationsSchema = z.object({
    available: z.boolean(),
    link: z.object({
        status: z.enum(["none", "linked", "blocked"]),
        needsRelink: z.boolean(),
        telegramUsername: z.string(),
        telegramName: z.string(),
        linkedAt: z.string().nullable(),
    }),
    notificationsEnabled: z.boolean(),
    dailySummaryTime: z.string(),
    timezone: z.string(),
    preferences: z.array(
        z.object({
            type: z.string(),
            group: z.string(),
            label: z.string(),
            description: z.string(),
            delivery: z.string(),
            active: z.boolean(),
            enabled: z.boolean(),
        }),
    ),
})

export const telegramLinkRequestSchema = z.object({
    url: z.string(),
    expiresAt: z.string(),
})

export type NotificationSettings = z.infer<typeof notificationSettingsSchema>
export type MyNotifications = z.infer<typeof myNotificationsSchema>
export type NotificationPreference = MyNotifications["preferences"][number]
export type TelegramLinkRequest = z.infer<typeof telegramLinkRequestSchema>

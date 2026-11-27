import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { apiClient } from "@/lib/api"
import {
    myNotificationsSchema,
    notificationSettingsSchema,
    telegramLinkRequestSchema,
    type MyNotifications,
} from "@/lib/schemas/notifications"

// Hooks de las notificaciones por Telegram (ADR 0012).

const SETTINGS_KEY = ["notifications", "settings"]
const MINE_KEY = ["notifications", "mine"]

export function useNotificationSettings() {
    return useQuery({
        queryKey: SETTINGS_KEY,
        retry: false,
        queryFn: async () => {
            const response = await apiClient.get("/api/notification-settings")
            return notificationSettingsSchema.parse(response.data)
        },
    })
}

export interface SaveNotificationSettingsInput {
    /** Vacío conserva el token guardado. */
    botToken: string
    isEnabled: boolean
    dailySummaryTime: string
    erpSilenceHours: number
    connectionCheckMinutes: number
}

export function useSaveNotificationSettings() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: SaveNotificationSettingsInput) => {
            const response = await apiClient.put("/api/notification-settings", input, { timeout: 30_000 })
            return notificationSettingsSchema.parse(response.data)
        },
        onSuccess: (settings) => {
            queryClient.setQueryData(SETTINGS_KEY, settings)
            queryClient.invalidateQueries({ queryKey: MINE_KEY })
        },
    })
}

/**
 * Los avisos del usuario de la sesión. Con waitingForLink consulta seguido,
 * porque la vinculación se completa en Telegram.
 */
export function useMyNotifications(waitingForLink: boolean) {
    return useQuery({
        queryKey: MINE_KEY,
        retry: false,
        refetchInterval: waitingForLink ? 3_000 : false,
        // El usuario está en Telegram mientras se vincula.
        refetchIntervalInBackground: waitingForLink,
        queryFn: async () => {
            const response = await apiClient.get("/api/me/notifications")
            return myNotificationsSchema.parse(response.data)
        },
    })
}

export function useStartTelegramLink() {
    return useMutation({
        mutationFn: async () => {
            const response = await apiClient.post("/api/me/notifications/telegram/link")
            return telegramLinkRequestSchema.parse(response.data)
        },
    })
}

export function useUnlinkTelegram() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async () => {
            await apiClient.delete("/api/me/notifications/telegram/link")
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: MINE_KEY }),
    })
}

export function useSendTelegramTest() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async () => {
            await apiClient.post("/api/me/notifications/telegram/test", undefined, { timeout: 30_000 })
        },
        // El envío puede descubrir que el bot está bloqueado.
        onSettled: () => queryClient.invalidateQueries({ queryKey: MINE_KEY }),
    })
}

export function useSetNotificationsEnabled() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (enabled: boolean) => {
            await apiClient.patch("/api/me/notifications", { enabled })
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: MINE_KEY }),
    })
}

export function useSaveNotificationPreference() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async ({ type, enabled }: { type: string; enabled: boolean }) => {
            await apiClient.put("/api/me/notifications/preferences", { preferences: { [type]: enabled } })
        },
        // La casilla cambia al instante; si falla, se recupera lo guardado.
        onMutate: async ({ type, enabled }) => {
            await queryClient.cancelQueries({ queryKey: MINE_KEY })
            queryClient.setQueryData<MyNotifications>(MINE_KEY, (current) =>
                current && {
                    ...current,
                    preferences: current.preferences.map((item) => (item.type === type ? { ...item, enabled } : item)),
                },
            )
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: MINE_KEY }),
    })
}

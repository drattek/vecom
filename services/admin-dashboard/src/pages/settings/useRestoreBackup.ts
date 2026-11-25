import { useMutation, useQueryClient } from "@tanstack/react-query"
import { z } from "zod"

import { apiClient } from "@/lib/api"

const restoreResultSchema = z.object({
    appId: z.string(),
    accessVerified: z.boolean(),
})

export const backupsKey = (connectionId: number) => ["channel-connections", "mercadolibre-backups", connectionId]

export function useRestoreBackup(connectionId: number, onRestored: () => void) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (backupId: number) => {
            const response = await apiClient.post(
                `/api/channel-connections/${connectionId}/mercadolibre/backups/${backupId}/restore`,
                {},
                // Incluye comprobar los tokens con MercadoLibre.
                { timeout: 60_000 },
            )
            return restoreResultSchema.parse(response.data)
        },
        // También al fallar: la restauración pudo aplicarse en el servidor.
        onSettled: () => {
            queryClient.invalidateQueries({ queryKey: backupsKey(connectionId) })
            queryClient.invalidateQueries({ queryKey: ["channel-connection-config", connectionId] })
            onRestored()
        },
    })
}

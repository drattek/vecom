import { apiClient } from "@/lib/api"
import { useMutation } from "@tanstack/react-query"
import { z } from "zod"

const enqueueCompatibilitiesResultSchema = z.object({
    enqueued: z.number(),
    requeued: z.number(),
})

export type EnqueueCompatibilitiesResult = z.infer<typeof enqueueCompatibilitiesResultSchema>

/**
 * useEnqueueConnectionCompatibilities encola (POST
 * /api/channel-listings/compatibilities/enqueue) todos los productos publicados
 * en la conexión para que el worker les empuje sus compatibilidades de
 * maquinaria y vehículos en segundo plano. La respuesta solo cuenta cuántos se
 * encolaron; el envío ocurre después.
 */
export function useEnqueueConnectionCompatibilities() {
    return useMutation({
        mutationFn: async (input: { connectionId: number }) => {
            const response = await apiClient.post("/api/channel-listings/compatibilities/enqueue", {
                connectionId: input.connectionId,
            })
            return enqueueCompatibilitiesResultSchema.parse(response.data)
        },
    })
}

/**
 * useEnqueueConnectionResync encola (POST /api/channel-listings/resync/enqueue)
 * todos los productos publicados en la conexión para que el worker les haga el
 * Resync completo en segundo plano (precio y stock se envían aunque no hayan
 * cambiado). Aplica a MercadoLibre y Odoo; la respuesta solo cuenta cuántos se
 * encolaron.
 */
export function useEnqueueConnectionResync() {
    return useMutation({
        mutationFn: async (input: { connectionId: number }) => {
            const response = await apiClient.post("/api/channel-listings/resync/enqueue", {
                connectionId: input.connectionId,
            })
            return enqueueCompatibilitiesResultSchema.parse(response.data)
        },
    })
}

import { apiClient } from "@/lib/api"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
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
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { connectionId: number }) => {
            const response = await apiClient.post("/api/channel-listings/compatibilities/enqueue", {
                connectionId: input.connectionId,
            })
            return enqueueCompatibilitiesResultSchema.parse(response.data)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["channel-listings", "queue"] }),
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
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { connectionId: number }) => {
            const response = await apiClient.post("/api/channel-listings/resync/enqueue", {
                connectionId: input.connectionId,
            })
            return enqueueCompatibilitiesResultSchema.parse(response.data)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["channel-listings", "queue"] }),
    })
}

const queueSummarySchema = z.object({
    syncType: z.string(),
    pending: z.number(),
    processing: z.number(),
    done: z.number(),
    failed: z.number(),
    lastProcessedAt: z.string().nullish(),
})

const queueEntrySchema = z.object({
    id: z.number(),
    productId: z.number(),
    sku: z.string(),
    productName: z.string(),
    syncType: z.string(),
    status: z.string(),
    attempts: z.number(),
    lastError: z.string().nullish(),
    requestedAt: z.string(),
    processedAt: z.string().nullish(),
    updatedAt: z.string(),
})

const queueViewSchema = z.object({
    summary: z.array(queueSummarySchema).nullish().transform((value) => value ?? []),
    entries: z.array(queueEntrySchema).nullish().transform((value) => value ?? []),
    total: z.number(),
})

export type QueueSummary = z.infer<typeof queueSummarySchema>
export type QueueEntry = z.infer<typeof queueEntrySchema>
export type QueueView = z.infer<typeof queueViewSchema>

const QUEUE_POLL_MS = 4000

/**
 * useConnectionQueue consulta (GET /api/channel-listings/queue) el avance de las
 * colas en segundo plano de una conexión: conteos por tipo y estado y, si se
 * pide syncType, las filas (con filtro de estado y paginación). Se refresca solo
 * mientras haya algo pendiente o procesándose.
 */
export function useConnectionQueue(
    connectionId: number,
    options: { syncType?: string; status?: string; offset?: number; pageSize?: number; enabled?: boolean } = {},
) {
    const { syncType, status, offset = 0, pageSize = 50, enabled = true } = options
    return useQuery({
        queryKey: ["channel-listings", "queue", connectionId, syncType ?? null, status ?? null, offset, pageSize],
        enabled,
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-listings/queue", {
                params: { connectionId, syncType, status, offset, pageSize },
            })
            return queueViewSchema.parse(response.data)
        },
        refetchInterval: (query) => {
            const active = query.state.data?.summary.some((item) => item.pending > 0 || item.processing > 0)
            return active ? QUEUE_POLL_MS : false
        },
    })
}

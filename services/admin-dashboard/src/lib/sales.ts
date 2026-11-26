import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { apiClient } from "@/lib/api"
import { paginatedSalesSchema, saleDetailSchema, salesSyncStatusSchema } from "@/lib/schemas/sales"

// Hooks y etiquetas de la sección "Ventas" (ADR 0010).

type StatusStyle = { label: string; tone: string }

const TONE_OK = "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
const TONE_WAIT = "bg-amber-500/10 text-amber-700 dark:text-amber-400"
const TONE_MOVING = "bg-sky-500/10 text-sky-700 dark:text-sky-400"
const TONE_BAD = "bg-red-500/10 text-red-700 dark:text-red-400"
const TONE_MUTED = "bg-muted text-muted-foreground"

const UNKNOWN: StatusStyle = { label: "Desconocido", tone: TONE_MUTED }

export const ORDER_STATUS: Record<string, StatusStyle> = {
    pending: { label: "Pendiente", tone: TONE_WAIT },
    confirmed: { label: "Confirmada", tone: TONE_OK },
    cancelled: { label: "Cancelada", tone: TONE_BAD },
    unknown: UNKNOWN,
}

export const PAYMENT_STATUS: Record<string, StatusStyle> = {
    pending: { label: "Pendiente", tone: TONE_WAIT },
    paid: { label: "Pagado", tone: TONE_OK },
    partially_refunded: { label: "Reembolso parcial", tone: TONE_WAIT },
    refunded: { label: "Reembolsado", tone: TONE_BAD },
    rejected: { label: "Rechazado", tone: TONE_BAD },
    unknown: UNKNOWN,
}

export const SHIPPING_STATUS: Record<string, StatusStyle> = {
    pending: { label: "Pendiente", tone: TONE_WAIT },
    ready_to_ship: { label: "Listo para enviar", tone: TONE_MOVING },
    shipped: { label: "Enviado", tone: TONE_MOVING },
    delivered: { label: "Entregado", tone: TONE_OK },
    not_delivered: { label: "No entregado", tone: TONE_BAD },
    cancelled: { label: "Cancelado", tone: TONE_BAD },
    not_applicable: { label: "No aplica", tone: TONE_MUTED },
    unknown: UNKNOWN,
}

export function statusStyle(statuses: Record<string, StatusStyle>, value: string): StatusStyle {
    return statuses[value] ?? UNKNOWN
}

/** Importe en la moneda de la venta. Un valor desconocido se muestra como "—". */
export function formatAmount(value: number | null, currencyCode: string | null): string {
    if (value === null) return "—"
    const formatted = value.toLocaleString("es-MX", { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    return currencyCode ? `$${formatted} ${currencyCode}` : `$${formatted}`
}

export type SalesFilters = {
    channelCode?: string
    connectionId?: string
    orderStatus?: string
    paymentStatus?: string
    shippingStatus?: string
    from?: string
    to?: string
    search?: string
}

/**
 * Estado de la sincronización por conexión. Se consulta seguido mientras hay
 * una corrida en curso.
 */
export function useSalesSyncStatus() {
    return useQuery({
        queryKey: ["sales", "sync-status"],
        retry: false,
        refetchInterval: (query) => (query.state.data?.connections.some((c) => c.running) ? 5_000 : 60_000),
        queryFn: async () => {
            const response = await apiClient.get("/api/sales/sync-status")
            return salesSyncStatusSchema.parse(response.data)
        },
    })
}

/** revision cambia cuando termina una corrida, para recargar la lista. */
export function useSales(filters: SalesFilters, offset: number, pageSize: number, revision: string) {
    return useQuery({
        queryKey: ["sales", "list", filters, offset, pageSize, revision],
        placeholderData: keepPreviousData,
        retry: false,
        queryFn: async () => {
            const response = await apiClient.get("/api/sales", {
                params: { ...filters, offset, pageSize },
                timeout: 30_000,
            })
            return paginatedSalesSchema.parse(response.data)
        },
    })
}

export function useSale(id: number | null) {
    return useQuery({
        queryKey: ["sales", "detail", id],
        enabled: id !== null,
        retry: false,
        queryFn: async () => {
            const response = await apiClient.get(`/api/sales/${id}`)
            return saleDetailSchema.parse(response.data)
        },
    })
}

/** "Sincronizar ahora": sin connectionId sincroniza todas las activas. */
export function useRequestSalesSync() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (connectionId?: number) => {
            await apiClient.post("/api/sales/sync", connectionId ? { connectionId } : {})
        },
        onSettled: () => {
            // La corrida se lanza en segundo plano; se espera un momento para
            // que el estado ya la muestre en curso.
            window.setTimeout(() => {
                queryClient.invalidateQueries({ queryKey: ["sales", "sync-status"] })
            }, 1_000)
        },
    })
}

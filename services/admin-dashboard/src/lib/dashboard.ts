import { keepPreviousData, useQuery } from "@tanstack/react-query"

import { apiClient } from "@/lib/api"
import {
    attentionPageSchema,
    catalogCountSchema,
    catalogPageSchema,
    connectionsHealthSchema,
    inventoryHealthSchema,
    queueHealthSchema,
    type AttentionState,
    type CatalogFilter,
} from "@/lib/schemas/dashboard"

// Cada bloque del Dashboard tiene su propia consulta, así que una lenta o
// fallida no detiene a las demás. El timeout global de 12 s del cliente se
// queda corto para los conteos sobre todo el catálogo.
const DASHBOARD_TIMEOUT_MS = 60_000
const STALE_MS = 60_000

async function fetchDashboard(path: string, signal: AbortSignal, params?: Record<string, string | number | undefined>) {
    const response = await apiClient.get(`/api/dashboard/${path}`, { params, signal, timeout: DASHBOARD_TIMEOUT_MS })
    return response.data as unknown
}

export function formatDateTime(value: string | null | undefined): string {
    if (!value) return "—"
    return new Date(value).toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

// keepPreviousData solo sostiene la tabla al cambiar de página: cada tabla
// tiene un estado fijo o se remonta al cambiar de filtro, así que nunca muestra
// filas de otro filtro.
export function useAttentionPage(state: AttentionState, offset: number, pageSize: number) {
    return useQuery({
        queryKey: ["dashboard", "listings", state, offset, pageSize],
        staleTime: STALE_MS,
        retry: false,
        placeholderData: keepPreviousData,
        queryFn: async ({ signal }) =>
            attentionPageSchema.parse(await fetchDashboard("listings", signal, { state, offset, pageSize })),
    })
}

export function useCatalogCount(filter: CatalogFilter, connectionId: number | null, enabled = true) {
    return useQuery({
        queryKey: ["dashboard", "catalog-count", filter, connectionId],
        enabled,
        staleTime: STALE_MS,
        retry: false,
        queryFn: async ({ signal }) =>
            catalogCountSchema.parse(
                await fetchDashboard("catalog/count", signal, { filter, connectionId: connectionId ?? undefined }),
            ).total,
    })
}

export function useCatalogProducts(filter: CatalogFilter, connectionId: number | null, offset: number, pageSize: number) {
    return useQuery({
        queryKey: ["dashboard", "catalog-products", filter, connectionId, offset, pageSize],
        staleTime: STALE_MS,
        retry: false,
        placeholderData: keepPreviousData,
        queryFn: async ({ signal }) =>
            catalogPageSchema.parse(
                await fetchDashboard("catalog/products", signal, {
                    filter,
                    connectionId: connectionId ?? undefined,
                    offset,
                    pageSize,
                }),
            ),
    })
}

export function useConnectionsHealth() {
    return useQuery({
        queryKey: ["dashboard", "health", "connections"],
        staleTime: STALE_MS,
        retry: false,
        queryFn: async ({ signal }) =>
            connectionsHealthSchema.parse(await fetchDashboard("health/connections", signal)).connections,
    })
}

export function useInventoryHealth() {
    return useQuery({
        queryKey: ["dashboard", "health", "inventory"],
        staleTime: STALE_MS,
        retry: false,
        queryFn: async ({ signal }) =>
            inventoryHealthSchema.parse(await fetchDashboard("health/inventory", signal)).sources,
    })
}

export function useQueueHealth() {
    return useQuery({
        queryKey: ["dashboard", "health", "queue"],
        staleTime: STALE_MS,
        retry: false,
        queryFn: async ({ signal }) => queueHealthSchema.parse(await fetchDashboard("health/queue", signal)).queues,
    })
}

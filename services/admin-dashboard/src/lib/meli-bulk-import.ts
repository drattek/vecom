import { apiClient } from "@/lib/api"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { z } from "zod"

// Importación masiva de publicaciones de MercadoLibre por Excel (Productos →
// Importar). El servidor encola cada fila (sku, meli_id) y un worker las
// importa de una en una; aquí solo se sube el archivo y se consulta el avance.

export const bulkImportStatusSchema = z.enum(["pending", "running", "done", "failed", "missing", "duplicate"])
export type BulkImportStatus = z.infer<typeof bulkImportStatusSchema>

const bulkImportStepSchema = z.object({
    step: z.string(),
    status: z.string(),
    detail: z.string().nullish(),
})

export const bulkImportItemSchema = z.object({
    id: z.number(),
    batchId: z.number(),
    batchFile: z.string(),
    connectionId: z.number(),
    sheetRow: z.number(),
    sku: z.string(),
    meliId: z.string(),
    externalId: z.string().nullish(),
    productId: z.number().nullish(),
    productName: z.string().nullish(),
    status: bulkImportStatusSchema,
    errorMessage: z.string().nullish(),
    listingTitle: z.string().nullish(),
    steps: z.array(bulkImportStepSchema).nullish(),
    attempts: z.number(),
    startedAt: z.string().nullish(),
    finishedAt: z.string().nullish(),
    createdAt: z.string(),
})
export type BulkImportItem = z.infer<typeof bulkImportItemSchema>

const bulkImportPageSchema = z.object({
    items: z.array(bulkImportItemSchema),
    total: z.number(),
    offset: z.number(),
    pageSize: z.number(),
})

const bulkImportSummarySchema = z.record(z.string(), z.number())

const uploadResultSchema = z.object({
    batchId: z.number(),
    totalRows: z.number(),
    queued: z.number(),
    missing: z.number(),
    invalid: z.number(),
})
export type BulkImportUploadResult = z.infer<typeof uploadResultSchema>

// Mientras haya filas en cola o importándose, la tabla se refresca sola.
const POLL_MS = 3_000
const UPLOAD_TIMEOUT_MS = 120_000

export type BulkImportQuery = {
    statuses: BulkImportStatus[]
    connectionId: number | null
    search: string
    history: boolean
    offset: number
    pageSize: number
}

export function useBulkImportSummary(connectionId: number | null) {
    return useQuery({
        queryKey: ["meli-bulk-import", "summary", connectionId],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/meli-bulk-import/summary", {
                params: connectionId ? { connectionId } : {},
            })
            return bulkImportSummarySchema.parse(response.data)
        },
        refetchInterval: (query) => {
            const counts = query.state.data
            return counts && ((counts.pending ?? 0) > 0 || (counts.running ?? 0) > 0) ? POLL_MS : false
        },
    })
}

export function useBulkImportItems(query: BulkImportQuery, polling: boolean) {
    return useQuery({
        queryKey: ["meli-bulk-import", "items", query],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/meli-bulk-import/items", {
                params: {
                    ...(query.statuses.length ? { status: query.statuses.join(",") } : {}),
                    ...(query.connectionId ? { connectionId: query.connectionId } : {}),
                    ...(query.search ? { search: query.search } : {}),
                    ...(query.history ? { history: true } : {}),
                    offset: query.offset,
                    pageSize: query.pageSize,
                },
            })
            return bulkImportPageSchema.parse(response.data)
        },
        refetchInterval: polling ? POLL_MS : false,
    })
}

export function useUploadBulkImport() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { connectionId: number; file: File }) => {
            const form = new FormData()
            form.append("connectionId", String(input.connectionId))
            form.append("file", input.file)
            const response = await apiClient.post("/api/meli-bulk-import/batches", form, { timeout: UPLOAD_TIMEOUT_MS })
            return uploadResultSchema.parse(response.data)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["meli-bulk-import"] }),
    })
}

export function useRetryBulkImportItem() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (itemId: number) => {
            await apiClient.post(`/api/meli-bulk-import/items/${itemId}/retry`)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["meli-bulk-import"] }),
    })
}

export function useRetryBulkImportAll() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { status: "failed" | "missing"; connectionId: number | null }) => {
            const response = await apiClient.post("/api/meli-bulk-import/retry", {
                status: input.status,
                connectionId: input.connectionId ?? 0,
            })
            return z.object({ requeued: z.number() }).parse(response.data).requeued
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["meli-bulk-import"] }),
    })
}

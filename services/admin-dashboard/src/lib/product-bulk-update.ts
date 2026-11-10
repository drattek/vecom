import { apiClient } from "@/lib/api"
import { sourcesResponseSchema } from "@/lib/schemas/publications-report"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { z } from "zod"

// Actualización masiva de productos por Excel (Productos → Actualización
// masiva). El servidor encola cada fila (sku, marca, título, descripciones,
// fotos, video y medidas) y un worker las aplica en segundo plano; aquí solo se
// sube el archivo y se consulta el avance, igual que en la importación de
// MercadoLibre (meli-bulk-import.ts).

export const productBulkUpdateStatusSchema = z.enum(["pending", "running", "done", "failed", "missing"])
export type ProductBulkUpdateStatus = z.infer<typeof productBulkUpdateStatusSchema>

const productBulkUpdateStepSchema = z.object({
    step: z.string(),
    status: z.string(),
    detail: z.string().nullish(),
})

export const productBulkUpdateItemSchema = z.object({
    id: z.number(),
    batchId: z.number(),
    batchFile: z.string(),
    sheetRow: z.number(),
    sku: z.string(),
    productId: z.number().nullish(),
    productName: z.string().nullish(),
    status: productBulkUpdateStatusSchema,
    errorMessage: z.string().nullish(),
    steps: z.array(productBulkUpdateStepSchema).nullish(),
    attempts: z.number(),
    startedAt: z.string().nullish(),
    finishedAt: z.string().nullish(),
    createdAt: z.string(),
    retryable: z.boolean(),
})
export type ProductBulkUpdateItem = z.infer<typeof productBulkUpdateItemSchema>

const productBulkUpdatePageSchema = z.object({
    items: z.array(productBulkUpdateItemSchema),
    total: z.number(),
    offset: z.number(),
    pageSize: z.number(),
})

const summarySchema = z.record(z.string(), z.number())

const uploadResultSchema = z.object({
    batchId: z.number(),
    totalRows: z.number(),
    queued: z.number(),
    missing: z.number(),
    invalid: z.number(),
})
export type ProductBulkUpdateUploadResult = z.infer<typeof uploadResultSchema>

const batchSchema = z.object({
    id: z.number(),
    sourceId: z.number(),
    storageDiskId: z.number(),
    syncConnectionIds: z.array(z.number()).nullish(),
    fileName: z.string(),
    totalRows: z.number(),
    createdAt: z.string(),
})
export type ProductBulkUpdateBatch = z.infer<typeof batchSchema>

// Mientras haya filas en cola o procesándose, la tabla se refresca sola.
const POLL_MS = 3_000
const UPLOAD_TIMEOUT_MS = 120_000

export type ProductBulkUpdateQuery = {
    statuses: ProductBulkUpdateStatus[]
    search: string
    history: boolean
    offset: number
    pageSize: number
}

const QUERY_KEY = "product-bulk-update"

export function useProductBulkUpdateSummary() {
    return useQuery({
        queryKey: [QUERY_KEY, "summary"],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/product-bulk-update/summary")
            return summarySchema.parse(response.data)
        },
        refetchInterval: (query) => {
            const counts = query.state.data
            return counts && ((counts.pending ?? 0) > 0 || (counts.running ?? 0) > 0) ? POLL_MS : false
        },
    })
}

export function useProductBulkUpdateItems(query: ProductBulkUpdateQuery, polling: boolean) {
    return useQuery({
        queryKey: [QUERY_KEY, "items", query],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/product-bulk-update/items", {
                params: {
                    ...(query.statuses.length ? { status: query.statuses.join(",") } : {}),
                    ...(query.search ? { search: query.search } : {}),
                    ...(query.history ? { history: true } : {}),
                    offset: query.offset,
                    pageSize: query.pageSize,
                },
            })
            return productBulkUpdatePageSchema.parse(response.data)
        },
        refetchInterval: polling ? POLL_MS : false,
    })
}

export function useProductBulkUpdateBatches() {
    return useQuery({
        queryKey: [QUERY_KEY, "batches"],
        queryFn: async () => {
            const response = await apiClient.get("/api/product-bulk-update/batches")
            return z.object({ batches: z.array(batchSchema) }).parse(response.data).batches
        },
    })
}

/** Sources (ecom_sources) para elegir de cuál son los productos del Excel. */
export function useProductSources() {
    return useQuery({
        queryKey: ["report-sources"],
        staleTime: 60_000,
        queryFn: async () => {
            const response = await apiClient.get("/api/sources", { params: { offset: 0, pageSize: 100 } })
            return sourcesResponseSchema.parse(response.data).sources
        },
    })
}

function saveBlob(blob: Blob, fileName: string) {
    const url = URL.createObjectURL(blob)
    const link = document.createElement("a")
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
}

function fileNameFrom(disposition: unknown, fallback: string): string {
    return /filename="([^"]+)"/.exec(String(disposition ?? ""))?.[1] ?? fallback
}

/** Descarga el reporte de un lote: estado final y resultado de cada paso por fila. */
export function useExportProductBulkUpdateBatch() {
    return useMutation({
        mutationFn: async (batchId: number) => {
            const response = await apiClient.get<Blob>(`/api/product-bulk-update/batches/${batchId}/export`, {
                responseType: "blob",
                timeout: UPLOAD_TIMEOUT_MS,
            })
            saveBlob(response.data, fileNameFrom(response.headers["content-disposition"], `reporte-lote-${batchId}.xlsx`))
        },
    })
}

/** Descarga la plantilla .xlsx con las columnas que lee el importador. */
export function useDownloadProductBulkUpdateTemplate() {
    return useMutation({
        mutationFn: async () => {
            const response = await apiClient.get<Blob>("/api/product-bulk-update/template", { responseType: "blob" })
            saveBlob(response.data, fileNameFrom(response.headers["content-disposition"], "plantilla-actualizacion-masiva.xlsx"))
        },
    })
}

export function useUploadProductBulkUpdate() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { sourceId: number; storageDiskId: number; syncConnectionIds: number[]; file: File }) => {
            const form = new FormData()
            form.append("sourceId", String(input.sourceId))
            form.append("storageDiskId", String(input.storageDiskId))
            // Conexiones donde resincronizar cada producto tras actualizarlo (opcional).
            for (const id of input.syncConnectionIds) form.append("syncConnectionIds", String(id))
            form.append("file", input.file)
            const response = await apiClient.post("/api/product-bulk-update/batches", form, { timeout: UPLOAD_TIMEOUT_MS })
            return uploadResultSchema.parse(response.data)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: [QUERY_KEY] }),
    })
}

export function useRetryProductBulkUpdateItem() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (itemId: number) => {
            await apiClient.post(`/api/product-bulk-update/items/${itemId}/retry`)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: [QUERY_KEY] }),
    })
}

export function useRetryProductBulkUpdateAll() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { status: "failed" | "missing" }) => {
            const response = await apiClient.post("/api/product-bulk-update/retry", { status: input.status, batchId: 0 })
            return z.object({ requeued: z.number() }).parse(response.data).requeued
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: [QUERY_KEY] }),
    })
}

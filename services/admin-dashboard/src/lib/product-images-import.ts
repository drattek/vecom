import { useMutation, useQueryClient } from "@tanstack/react-query"
import { z } from "zod"
import { apiClient } from "@/lib/api"

/** Un SKU con sus URLs de imagen, en el orden en que aparecen en el Excel. */
export type SkuImageGroup = {
    sku: string
    urls: string[]
}

export type ParsedImagesSheet = {
    groups: SkuImageGroup[]
    /** Filas con datos que se descartaron (falta sku o image_url). */
    skippedRows: number
    /** URLs repetidas para el mismo SKU, descartadas. */
    duplicateUrls: number
}

function cellToString(value: unknown): string {
    if (value == null) return ""
    return String(value).trim()
}

/**
 * Lee el primer sheet de un .xlsx con columna A = sku y columna B = image_url
 * y agrupa las URLs por SKU. La primera fila se ignora si es el encabezado.
 */
export async function parseImagesSheet(file: File): Promise<ParsedImagesSheet> {
    // Import dinámico: la librería solo se descarga al usar esta vista.
    const { readSheet } = await import("read-excel-file/browser")
    const rows = await readSheet(file)

    const bySku = new Map<string, string[]>()
    let skippedRows = 0
    let duplicateUrls = 0

    rows.forEach((row, index) => {
        const sku = cellToString(row[0])
        const url = cellToString(row[1])

        if (index === 0 && sku.toLowerCase() === "sku") return
        if (!sku && !url) return
        if (!sku || !url) {
            skippedRows++
            return
        }

        const urls = bySku.get(sku) ?? []
        if (urls.includes(url)) {
            duplicateUrls++
            return
        }
        urls.push(url)
        bySku.set(sku, urls)
    })

    return {
        groups: Array.from(bySku, ([sku, urls]) => ({ sku, urls })),
        skippedRows,
        duplicateUrls,
    }
}

const importResultSchema = z.object({
    sku: z.string().optional().default(""),
    url: z.string().optional().default(""),
    success: z.boolean(),
    error: z.string().nullish(),
})

const importResponseSchema = z.object({
    results: z.array(importResultSchema),
})

export type ImageImportItemResult = z.infer<typeof importResultSchema>

/** SKUs por request: el backend valida cada URL con un HEAD, así que se sube por lotes. */
const BATCH_SIZE = 10

/**
 * Sube los grupos a POST /api/product-images/import por lotes, reportando el
 * avance. Un lote que falla completo se marca como error para sus SKUs sin
 * frenar el resto.
 */
export function useImportImagesBySku() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: {
            storageDiskId: number
            groups: SkuImageGroup[]
            onProgress?: (done: number, total: number) => void
        }): Promise<ImageImportItemResult[]> => {
            const results: ImageImportItemResult[] = []
            for (let i = 0; i < input.groups.length; i += BATCH_SIZE) {
                const batch = input.groups.slice(i, i + BATCH_SIZE)
                try {
                    const response = await apiClient.post("/api/product-images/import", {
                        storageDiskId: input.storageDiskId,
                        products: batch,
                    })
                    results.push(...importResponseSchema.parse(response.data).results)
                } catch (error) {
                    const message = error instanceof Error ? error.message : "Error desconocido"
                    for (const group of batch) {
                        for (const url of group.urls) {
                            results.push({ sku: group.sku, url, success: false, error: message })
                        }
                    }
                }
                input.onProgress?.(Math.min(i + BATCH_SIZE, input.groups.length), input.groups.length)
            }
            return results
        },
        onSettled: () => {
            queryClient.invalidateQueries({ queryKey: ["products"] })
        },
    })
}

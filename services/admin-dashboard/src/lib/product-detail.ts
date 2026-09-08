import { apiClient, getServerErrorMessage } from "@/lib/api"
import { categoriesResponseSchema, type CategoryTree } from "@/lib/schemas/categories"
import {
    paginatedChannelConnectionsResponseSchema,
    type ChannelConnection,
} from "@/lib/schemas/channel-connections"
import {
    attributeChecklistSchema,
    brandOptionsResponseSchema,
    productAttributesSectionSchema,
    productGeneralSchema,
    productAttachmentImportResponseSchema,
    productAttachmentImportResultSchema,
    productImageImportResponseSchema,
    productImageImportResultSchema,
    productVideoImportResponseSchema,
    productVideoImportResultSchema,
    type BrandOption,
    type ProductDimensionsPatch,
    type ProductGeneralPatch,
    type ProductSeoPatch,
} from "@/lib/schemas/product-details"
import {
    paginatedStorageDisksResponseSchema,
    type StorageDisk,
} from "@/lib/schemas/storage-disk"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import type { ZodType } from "zod"

// Hook y formateadores compartidos por las secciones del detalle de producto.
// Viven aparte de ProductDetailPrimitives.tsx porque ese archivo solo exporta
// componentes (requisito de react-refresh para que funcione el fast refresh).

/**
 * useProductSection consulta un endpoint /api/products/{id}/details/{section}
 * y valida la respuesta con su schema. Cada sección se pide por separado, así
 * que abrir el detalle solo trae la pestaña que se está viendo.
 */
export function useProductSection<T>(productId: string | undefined, section: string, schema: ZodType<T>) {
    return useQuery({
        queryKey: ["product-details", productId, section],
        enabled: Boolean(productId),
        queryFn: async () => {
            const response = await apiClient.get(`/api/products/${productId}/details/${section}`)
            return schema.parse(response.data)
        },
    })
}

/**
 * useProductSectionPage es como useProductSection pero para las sub-tablas
 * paginadas del detalle (movimientos de stock, historial de precio): agrega
 * offset/pageSize al query string y mantiene la página anterior visible mientras
 * carga la siguiente (keepPreviousData) para que la tabla no parpadee al paginar.
 */
export function useProductSectionPage<T>(
    productId: string | undefined,
    section: string,
    schema: ZodType<T>,
    page: { offset: number; pageSize: number },
) {
    return useQuery({
        queryKey: ["product-details", productId, section, page.offset, page.pageSize],
        enabled: Boolean(productId),
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get(`/api/products/${productId}/details/${section}`, {
                params: { offset: page.offset, pageSize: page.pageSize },
            })
            return schema.parse(response.data)
        },
    })
}

/**
 * useUpdateProductGeneral aplica una edición inline de la sección General
 * (PATCH /api/products/{id}/details/general). El patch es parcial: cada card
 * manda solo las claves que edita. La respuesta trae el DTO General fresco, con
 * el que se refresca la caché — la misma queryKey que consume la cabecera de
 * ProductDetailPage, así que el título también se actualiza.
 */
export function useUpdateProductGeneral(productId: string | undefined) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (patch: ProductGeneralPatch) => {
            const response = await apiClient.patch(`/api/products/${productId}/details/general`, patch)
            return productGeneralSchema.parse(response.data)
        },
        onSuccess: (data) => {
            queryClient.setQueryData(["product-details", productId, "general"], data)
        },
    })
}

/**
 * useUpdateProductDimensions hace un upsert de la sección Dimensiones
 * (PATCH /api/products/{id}/details/attributes/dimensions) y refresca la caché
 * de la sección Atributos con la respuesta.
 */
export function useUpdateProductDimensions(productId: string | undefined) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (patch: ProductDimensionsPatch) => {
            const response = await apiClient.patch(
                `/api/products/${productId}/details/attributes/dimensions`,
                patch,
            )
            return productAttributesSectionSchema.parse(response.data)
        },
        onSuccess: (data) => {
            queryClient.setQueryData(["product-details", productId, "attributes"], data)
        },
    })
}

/**
 * useUpdateProductSeo hace un upsert de la sección SEO
 * (PATCH /api/products/{id}/details/attributes/seo) y refresca la caché de la
 * sección Atributos con la respuesta.
 */
export function useUpdateProductSeo(productId: string | undefined) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (patch: ProductSeoPatch) => {
            const response = await apiClient.patch(
                `/api/products/${productId}/details/attributes/seo`,
                patch,
            )
            return productAttributesSectionSchema.parse(response.data)
        },
        onSuccess: (data) => {
            queryClient.setQueryData(["product-details", productId, "attributes"], data)
        },
    })
}

// --- Checklist de atributos por canal --------------------------------------

/** Conexiones de canal para el selector del checklist de atributos. */
export function useChannelConnections() {
    return useQuery({
        queryKey: ["channel-connections", "all"],
        staleTime: 5 * 60_000,
        queryFn: async (): Promise<ChannelConnection[]> => {
            const response = await apiClient.get("/api/channel-connections", { params: { pageSize: 100 } })
            return paginatedChannelConnectionsResponseSchema.parse(response.data).channelConnections
        },
    })
}

/**
 * useAttributeChecklist trae, para el producto + conexión, qué atributos espera
 * el canal para su categoría (requerido/opcional + valor actual). Solo corre
 * con una conexión elegida.
 */
export function useAttributeChecklist(productId: string | undefined, connectionId: number | null) {
    return useQuery({
        queryKey: ["product-details", productId, "attributes", "checklist", connectionId],
        enabled: Boolean(productId) && connectionId != null,
        refetchOnWindowFocus: false,
        queryFn: async () => {
            const response = await apiClient.get(
                `/api/products/${productId}/details/attributes/checklist`,
                { params: { connectionId } },
            )
            return attributeChecklistSchema.parse(response.data)
        },
    })
}

type SetProductAttributeInput = {
    attributeId: number
    valueText?: string
    valueNumber?: number
    valueBoolean?: boolean
    valueDate?: string
    optionId?: number
}

/** Guarda el valor de un atributo del producto (PUT /api/products/{id}/attributes). */
export function useSetProductAttribute(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: SetProductAttributeInput) => {
            await apiClient.put(`/api/products/${productId}/attributes`, input)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["product-details", productId, "attributes"] })
        },
    })
}

/** Borra el valor de un atributo del producto (DELETE /api/product-attributes/{id}). */
export function useDeleteProductAttribute(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (productAttributeId: number) => {
            await apiClient.delete(`/api/product-attributes/${productAttributeId}`)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["product-details", productId, "attributes"] })
        },
    })
}

// --- Multimedia: imágenes por URL ----------------------------------------

/** Discos de almacenamiento para el selector del diálogo "Agregar imágenes". */
export function useStorageDisks() {
    return useQuery({
        queryKey: ["storage-disks", "all"],
        staleTime: 5 * 60_000,
        queryFn: async (): Promise<StorageDisk[]> => {
            const response = await apiClient.get("/api/storage-disks", { params: { pageSize: 100 } })
            return paginatedStorageDisksResponseSchema.parse(response.data).storageDisks
        },
    })
}

/**
 * Invalida las cachés que dependen de las imágenes del producto: la sección
 * Multimedia y la sección General (la portada del encabezado sale del DTO
 * General).
 */
function invalidateProductImages(queryClient: ReturnType<typeof useQueryClient>, productId: string | undefined) {
    queryClient.invalidateQueries({ queryKey: ["product-details", productId, "media"] })
    queryClient.invalidateQueries({ queryKey: ["product-details", productId, "general"] })
}

type AddProductImagesInput = {
    storageDiskId: number
    images: { url: string; isFirst: boolean }[]
}

/** Alta masiva de imágenes por URL (POST /api/products/{id}/images/bulk). */
export function useAddProductImages(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: AddProductImagesInput) => {
            const response = await apiClient.post(`/api/products/${productId}/images/bulk`, input)
            return productImageImportResponseSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductImages(queryClient, productId),
    })
}

/** Marca una imagen existente como portada (PUT .../images/{imageId}/cover). */
export function useSetProductImageCover(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (imageId: number) => {
            await apiClient.put(`/api/products/${productId}/images/${imageId}/cover`)
        },
        onSuccess: () => invalidateProductImages(queryClient, productId),
    })
}

/** Reemplaza el archivo de una imagen por otra URL (PUT .../images/{imageId}/replace). */
export function useReplaceProductImage(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { imageId: number; storageDiskId: number; url: string }) => {
            const response = await apiClient.put(
                `/api/products/${productId}/images/${input.imageId}/replace`,
                { storageDiskId: input.storageDiskId, url: input.url },
            )
            return productImageImportResultSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductImages(queryClient, productId),
    })
}

/** Elimina (soft delete) una imagen del producto (DELETE /api/product-images/{imageId}). */
export function useDeleteProductImage(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (imageId: number) => {
            await apiClient.delete(`/api/product-images/${imageId}`)
        },
        onSuccess: () => invalidateProductImages(queryClient, productId),
    })
}

// --- Multimedia: archivos adjuntos por URL (ecom_product_media) -----------

function invalidateProductMedia(queryClient: ReturnType<typeof useQueryClient>, productId: string | undefined) {
    queryClient.invalidateQueries({ queryKey: ["product-details", productId, "media"] })
}

type AddProductAttachmentsInput = {
    storageDiskId: number
    attachments: { url: string; type: string }[]
}

/** Alta masiva de archivos adjuntos por URL (POST /api/products/{id}/attachments/bulk). */
export function useAddProductAttachments(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: AddProductAttachmentsInput) => {
            const response = await apiClient.post(`/api/products/${productId}/attachments/bulk`, input)
            return productAttachmentImportResponseSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

/** Reemplaza el archivo de un adjunto por otra URL (PUT .../attachments/{fileId}/replace). */
export function useReplaceProductAttachment(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { fileId: number; storageDiskId: number; url: string; type: string }) => {
            const response = await apiClient.put(
                `/api/products/${productId}/attachments/${input.fileId}/replace`,
                { storageDiskId: input.storageDiskId, url: input.url, type: input.type },
            )
            return productAttachmentImportResultSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

/** Elimina (soft delete) un adjunto del producto (DELETE .../attachments/{fileId}). */
export function useDeleteProductAttachment(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (fileId: number) => {
            await apiClient.delete(`/api/products/${productId}/attachments/${fileId}`)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

// --- Multimedia: videos por URL (ecom_product_videos) --------------------

/** Alta masiva de videos por URL (POST /api/products/{id}/videos/bulk). */
export function useAddProductVideos(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { storageDiskId: number; urls: string[] }) => {
            const response = await apiClient.post(`/api/products/${productId}/videos/bulk`, input)
            return productVideoImportResponseSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

/** Reemplaza el archivo de un video por otra URL (PUT .../videos/{videoId}/replace). */
export function useReplaceProductVideo(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { videoId: number; storageDiskId: number; url: string }) => {
            const response = await apiClient.put(
                `/api/products/${productId}/videos/${input.videoId}/replace`,
                { storageDiskId: input.storageDiskId, url: input.url },
            )
            return productVideoImportResultSchema.parse(response.data)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

/** Elimina (soft delete) un video del producto (DELETE /api/product-videos/{videoId}). */
export function useDeleteProductVideo(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (videoId: number) => {
            await apiClient.delete(`/api/product-videos/${videoId}`)
        },
        onSuccess: () => invalidateProductMedia(queryClient, productId),
    })
}

// --- Números de parte alternativos ----------------------------------------

type AddAlternatePartNumberInput = { partNumber: string; brandId: number; type: string }

/** Agrega un número de parte alternativo (POST /api/product-part-numbers). */
export function useAddAlternatePartNumber(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: AddAlternatePartNumberInput) => {
            await apiClient.post("/api/product-part-numbers", {
                productId: Number(productId),
                partNumber: input.partNumber,
                brandId: input.brandId,
                type: input.type,
            })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["product-details", productId, "part-numbers"] })
        },
    })
}

/** Quita un número de parte alternativo (DELETE /api/products/{id}/part-numbers/{pn}). */
export function useDeleteAlternatePartNumber(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (partNumber: string) => {
            await apiClient.delete(
                `/api/products/${productId}/part-numbers/${encodeURIComponent(partNumber)}`,
            )
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["product-details", productId, "part-numbers"] })
        },
    })
}

/** Resultado de la validación de un borrador antes de mandarlo al PATCH. */
export type CardEditorValidation<TPatch> =
    | { ok: true; patch: TPatch }
    | { ok: false; message: string }

/**
 * useCardEditor centraliza el ciclo de edición inline de una card del detalle:
 * toggle, borrador local, validación y guardado. Lo comparten las secciones
 * General y Atributos. `submit` normalmente es `mutation.mutateAsync`.
 */
export function useCardEditor<TDraft, TPatch>(config: {
    seed: () => TDraft
    isSaving: boolean
    validate: (draft: TDraft) => CardEditorValidation<TPatch>
    submit: (patch: TPatch) => Promise<unknown>
    onReset?: () => void
    fallbackErrorMessage?: string
}) {
    const [isEditing, setIsEditing] = useState(false)
    const [draft, setDraft] = useState<TDraft>(config.seed)
    const [error, setError] = useState<string | null>(null)

    return {
        isEditing,
        isSaving: config.isSaving,
        draft,
        setDraft,
        error,
        start() {
            setDraft(config.seed())
            setError(null)
            config.onReset?.()
            setIsEditing(true)
        },
        cancel() {
            setIsEditing(false)
            setError(null)
            config.onReset?.()
        },
        async save() {
            const result = config.validate(draft)
            if (!result.ok) {
                setError(result.message)
                return
            }
            setError(null)
            try {
                await config.submit(result.patch)
                setIsEditing(false)
            } catch (submitError) {
                setError(
                    getServerErrorMessage(
                        submitError,
                        config.fallbackErrorMessage ?? "No se pudieron guardar los cambios.",
                    ),
                )
            }
        },
    }
}

/**
 * CategoryNode enriquece el árbol de categorías con la ruta completa y si es
 * hoja (sin hijos) — solo las hojas son asignables a un producto.
 */
export type CategoryNode = {
    id: number
    name: string
    parentId: number | null
    path: string
    isLeaf: boolean
    children: CategoryNode[]
}

function decorateCategoryTree(
    nodes: CategoryTree[],
    parentPath: string,
    parentId: number | null,
): CategoryNode[] {
    return nodes.map((node) => {
        const path = parentPath ? `${parentPath} / ${node.name}` : node.name
        return {
            id: node.id,
            name: node.name,
            parentId,
            path,
            isLeaf: node.children.length === 0,
            children: decorateCategoryTree(node.children, path, node.id),
        }
    })
}

/**
 * useBrandOptions / useCategoryTree alimentan los selectores de la sección
 * Clasificación. Se cachean aparte (no dependen del producto) y con staleTime
 * alto porque los catálogos cambian poco.
 */
export function useBrandOptions(enabled = true) {
    return useQuery({
        queryKey: ["brand-options"],
        enabled,
        staleTime: 5 * 60_000,
        queryFn: async (): Promise<BrandOption[]> => {
            const response = await apiClient.get("/api/brands/options")
            return brandOptionsResponseSchema.parse(response.data).brands
        },
    })
}

/**
 * useCategoryTree devuelve el árbol decorado + un índice `byId` para resolver la
 * ruta de la categoría seleccionada sin recorrer el árbol.
 */
export function useCategoryTree(enabled = true) {
    return useQuery({
        queryKey: ["category-tree"],
        enabled,
        staleTime: 5 * 60_000,
        queryFn: async () => {
            const response = await apiClient.get("/api/categories")
            const { categories } = categoriesResponseSchema.parse(response.data)
            const tree = decorateCategoryTree(categories, "", null)

            const byId = new Map<number, CategoryNode>()
            const index = (nodes: CategoryNode[]) => {
                for (const node of nodes) {
                    byId.set(node.id, node)
                    index(node.children)
                }
            }
            index(tree)

            return { tree, byId }
        },
    })
}

/** Formatea una fecha ISO del API al formato local corto. "—" si no hay valor. */
export function formatDate(value?: string | null, withTime = false): string {
    if (!value) {
        return "—"
    }

    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
        return value
    }

    return date.toLocaleString("es-MX", {
        year: "numeric",
        month: "short",
        day: "2-digit",
        ...(withTime ? { hour: "2-digit", minute: "2-digit" } : {}),
    })
}

/** Formatea un monto decimal (llega como string desde MySQL) con su divisa. */
export function formatAmount(amount: string, symbol?: string | null, code?: string | null): string {
    const parsed = Number(amount)
    const formatted = Number.isFinite(parsed)
        ? parsed.toLocaleString("es-MX", { minimumFractionDigits: 2, maximumFractionDigits: 2 })
        : amount

    return `${symbol ?? ""}${formatted}${code ? ` ${code}` : ""}`
}

export function formatFileSize(bytes: number): string {
    if (bytes <= 0) {
        return "—"
    }
    if (bytes < 1024) {
        return `${bytes} B`
    }
    if (bytes < 1024 * 1024) {
        return `${(bytes / 1024).toFixed(1)} KB`
    }
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

import { apiClient } from "@/lib/api"
import {
    categoriesResponseSchema,
    categoryDetailSchema,
    categoryImportResultSchema,
    categoryImportSourcesResponseSchema,
    categoryImportTreeResponseSchema,
    setCategoryMappingResultSchema,
    type CategoryImportNode,
    type CategoryTree,
} from "@/lib/schemas/categories"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

// Hooks y helpers del catálogo de categorías locales + la importación desde
// canales. Viven aparte para que tanto la vista de árbol de Settings como el
// selector de padre reutilizable dependan de aquí y no de lib/product-detail.

/**
 * CategoryNode enriquece el árbol de categorías con la ruta completa y si es
 * hoja (sin hijos).
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
 * useCategoryTree devuelve el árbol decorado + un índice `byId` para resolver la
 * ruta de una categoría sin recorrer el árbol. Se cachea aparte y con staleTime
 * alto porque el catálogo cambia poco.
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

/** Invalida las dos cachés del catálogo: el árbol crudo y el decorado. */
function invalidateCategoryCaches(queryClient: ReturnType<typeof useQueryClient>) {
    queryClient.invalidateQueries({ queryKey: ["categories"] })
    queryClient.invalidateQueries({ queryKey: ["category-tree"] })
}

/** Alta de categoría (POST /api/categories). `parentId` null = categoría raíz. */
export function useCreateCategory() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { name: string; parentId: number | null }) => {
            const response = await apiClient.post("/api/categories", input)
            return categoryDetailSchema.parse(response.data)
        },
        onSuccess: () => invalidateCategoryCaches(queryClient),
    })
}

/**
 * Códigos de canal para los que el backend implementa navegar el árbol externo
 * e importar/vincular categorías (ver category_import.Service). El front lo usa
 * para decidir si ofrecer el botón de vincular en el detalle de categoría.
 */
export const IMPORT_SUPPORTED_CHANNEL_CODES = new Set(["MERCADOLIBRE", "ODOO"])

/** Canales con conexión activa para el modal de importar categoría. */
export function useCategoryImportSources() {
    return useQuery({
        queryKey: ["category-import-sources"],
        queryFn: async () => {
            const response = await apiClient.get("/api/category-import/sources")
            return categoryImportSourcesResponseSchema.parse(response.data).sources
        },
    })
}

/**
 * Trae un nivel del árbol de categorías externas de una conexión.
 * `externalCategoryId` vacío ⇒ raíces. No usa react-query: el modal cachea los
 * niveles ya cargados en estado local.
 */
export async function fetchCategoryImportTree(
    connectionId: number,
    externalCategoryId?: string,
): Promise<CategoryImportNode[]> {
    const response = await apiClient.get("/api/category-import/tree", {
        params: {
            connectionId,
            ...(externalCategoryId ? { externalCategoryId } : {}),
        },
    })
    return categoryImportTreeResponseSchema.parse(response.data).nodes
}

/** Importa una hoja externa (POST /api/category-import). */
export function useImportCategory() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: {
            connectionId: number
            externalCategoryId: string
            onlySelectedConnection: boolean
        }) => {
            const response = await apiClient.post("/api/category-import", input)
            return categoryImportResultSchema.parse(response.data)
        },
        onSuccess: () => {
            invalidateCategoryCaches(queryClient)
            queryClient.invalidateQueries({ queryKey: ["category-channel-mappings"] })
        },
    })
}

/**
 * Vincula (o reemplaza) el mapeo de una categoría local con una hoja externa en
 * una conexión — POST /api/category-import/mapping.
 */
export function useSetCategoryChannelMapping(categoryId: number | string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { connectionId: number; externalCategoryId: string }) => {
            const response = await apiClient.post("/api/category-import/mapping", {
                categoryId: Number(categoryId),
                connectionId: input.connectionId,
                externalCategoryId: input.externalCategoryId,
            })
            return setCategoryMappingResultSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["category-channel-mappings", String(categoryId)] })
        },
    })
}

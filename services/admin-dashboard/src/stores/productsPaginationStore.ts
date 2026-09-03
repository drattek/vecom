import { create } from "zustand"

export type ProductSortColumn = "sku" | "partNumber" | "name" | "brand" | "stock" | "price"
export type SortDirection = "asc" | "desc"

type ProductsPaginationState = {
    offset: number
    pageSize: number
    sortBy: ProductSortColumn | null
    sortDir: SortDirection
    search: string
    setOffset: (offset: number) => void
    setPageSize: (pageSize: number) => void
    setSearch: (search: string) => void
    toggleSort: (column: ProductSortColumn) => void
    reset: () => void
}

const DEFAULT_PAGE_SIZE = 10

export const useProductsPaginationStore = create<ProductsPaginationState>((set, get) => ({
    offset: 0,
    pageSize: DEFAULT_PAGE_SIZE,
    sortBy: null,
    sortDir: "asc",
    search: "",
    setOffset: (offset) => set({ offset: Math.max(0, offset) }),
    setPageSize: (pageSize) => set({ pageSize: pageSize > 0 ? pageSize : DEFAULT_PAGE_SIZE, offset: 0 }),
    setSearch: (search) => set({ search, offset: 0 }),
    // Ciclo de 3 estados por columna: sin ordenar -> asc -> desc -> sin ordenar.
    // Cambiar a otra columna siempre arranca en asc.
    toggleSort: (column) => {
        const { sortBy, sortDir } = get()

        if (sortBy !== column) {
            set({ sortBy: column, sortDir: "asc", offset: 0 })
            return
        }

        if (sortDir === "asc") {
            set({ sortDir: "desc", offset: 0 })
            return
        }

        set({ sortBy: null, sortDir: "asc", offset: 0 })
    },
    reset: () => set({ offset: 0, pageSize: DEFAULT_PAGE_SIZE, sortBy: null, sortDir: "asc", search: "" }),
}))

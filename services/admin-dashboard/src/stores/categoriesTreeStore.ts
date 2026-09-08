import { create } from "zustand"

// Estado de la vista de árbol de categorías (qué nodos están expandidos y el
// filtro de búsqueda). Vive en un store para que sobreviva a navegar al detalle
// de una categoría y volver. El árbol completo se carga de una sola vez
// (778 categorías ≈ 20 KB gzip), así que no hay carga perezosa de hijos.
type CategoriesTreeState = {
    expanded: Set<number>
    filter: string
    toggle: (id: number) => void
    setExpanded: (ids: Iterable<number>) => void
    collapseAll: () => void
    setFilter: (filter: string) => void
}

export const useCategoriesTreeStore = create<CategoriesTreeState>((set) => ({
    expanded: new Set(),
    filter: "",
    toggle: (id) =>
        set((state) => {
            const next = new Set(state.expanded)
            if (next.has(id)) {
                next.delete(id)
            } else {
                next.add(id)
            }
            return { expanded: next }
        }),
    setExpanded: (ids) => set({ expanded: new Set(ids) }),
    collapseAll: () => set({ expanded: new Set() }),
    setFilter: (filter) => set({ filter }),
}))

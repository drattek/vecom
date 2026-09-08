import { CreateCategoryDialog } from "@/components/categories/CreateCategoryDialog"
import { ImportCategoryDialog } from "@/components/categories/ImportCategoryDialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { apiClient } from "@/lib/api"
import { categoriesResponseSchema, type CategoryTree } from "@/lib/schemas/categories"
import { useCategoriesTreeStore } from "@/stores/categoriesTreeStore"
import { cn } from "@/lib/utils"
import { useQuery } from "@tanstack/react-query"
import { ChevronRightIcon, SearchIcon } from "lucide-react"
import { useMemo } from "react"
import { Link } from "react-router-dom"

type Filtered = {
    // Nodos que se renderizan: coincidencias + sus ancestros.
    visible: Set<number>
    // Nodos con descendientes visibles → se fuerzan expandidos.
    autoExpand: Set<number>
    matches: Set<number>
}

/** Calcula qué nodos mostrar para un filtro dado (null si el filtro está vacío). */
function computeFiltered(tree: CategoryTree[], rawQuery: string): Filtered | null {
    const query = rawQuery.trim().toLowerCase()
    if (query === "") {
        return null
    }

    const visible = new Set<number>()
    const autoExpand = new Set<number>()
    const matches = new Set<number>()

    const walk = (nodes: CategoryTree[]): boolean => {
        let anyVisible = false
        for (const node of nodes) {
            const selfMatch = node.name.toLowerCase().includes(query)
            const childVisible = walk(node.children)
            if (selfMatch || childVisible) {
                visible.add(node.id)
                anyVisible = true
                if (selfMatch) {
                    matches.add(node.id)
                }
                if (childVisible) {
                    autoExpand.add(node.id)
                }
            }
        }
        return anyVisible
    }

    walk(tree)
    return { visible, autoExpand, matches }
}

function countDescendants(node: CategoryTree): number {
    return node.children.reduce((sum, child) => sum + 1 + countDescendants(child), 0)
}

/** Resalta la coincidencia del filtro dentro del nombre. */
function highlight(name: string, rawQuery: string) {
    const query = rawQuery.trim()
    if (query === "") {
        return name
    }
    const index = name.toLowerCase().indexOf(query.toLowerCase())
    if (index === -1) {
        return name
    }
    return (
        <>
            {name.slice(0, index)}
            <mark className="rounded-sm bg-amber-500/20 text-foreground">
                {name.slice(index, index + query.length)}
            </mark>
            {name.slice(index + query.length)}
        </>
    )
}

export function CategoriesPage() {
    const filter = useCategoriesTreeStore((state) => state.filter)
    const setFilter = useCategoriesTreeStore((state) => state.setFilter)
    const expanded = useCategoriesTreeStore((state) => state.expanded)
    const toggle = useCategoriesTreeStore((state) => state.toggle)
    const collapseAll = useCategoriesTreeStore((state) => state.collapseAll)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["categories"],
        queryFn: async () => {
            const response = await apiClient.get("/api/categories")
            return categoriesResponseSchema.parse(response.data)
        },
    })

    const tree = useMemo(() => data?.categories ?? [], [data])
    const filtered = useMemo(() => computeFiltered(tree, filter), [tree, filter])
    const total = useMemo(
        () => tree.reduce((sum, node) => sum + 1 + countDescendants(node), 0),
        [tree],
    )

    const roots = filtered ? tree.filter((node) => filtered.visible.has(node.id)) : tree

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Categorías</h2>
            <p className="mt-2 text-sm text-muted-foreground">
                Jerarquía de categorías del catálogo{total > 0 ? ` · ${total} en total` : ""}. Selecciona un nombre
                para ver y editar su detalle.
            </p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las categorías: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <div className="mt-4 flex flex-wrap items-center gap-2">
                <div className="relative flex-1 sm:max-w-xs">
                    <SearchIcon className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={filter}
                        onChange={(event) => setFilter(event.target.value)}
                        placeholder="Buscar categoría…"
                        aria-label="Buscar categoría"
                        className="pl-8"
                    />
                </div>
                <Button variant="outline" size="sm" onClick={collapseAll} disabled={expanded.size === 0}>
                    Colapsar todo
                </Button>
                <div className="flex-1" />
                <CreateCategoryDialog />
                <ImportCategoryDialog />
            </div>

            <div className="mt-3 min-h-0 flex-1 overflow-auto rounded-lg border border-border">
                {isLoading ? (
                    <p className="py-8 text-center text-sm text-muted-foreground">Cargando categorías…</p>
                ) : roots.length === 0 ? (
                    <p className="py-8 text-center text-sm text-muted-foreground">
                        {filter.trim() !== "" ? "Ninguna categoría coincide con la búsqueda." : "No se encontraron categorías."}
                    </p>
                ) : (
                    <ul className="py-1 text-sm">
                        {roots.map((node) => (
                            <TreeNode
                                key={node.id}
                                node={node}
                                depth={0}
                                expanded={expanded}
                                onToggle={toggle}
                                filtered={filtered}
                                filter={filter}
                            />
                        ))}
                    </ul>
                )}
            </div>
        </section>
    )
}

function TreeNode({
    node,
    depth,
    expanded,
    onToggle,
    filtered,
    filter,
}: {
    node: CategoryTree
    depth: number
    expanded: Set<number>
    onToggle: (id: number) => void
    filtered: Filtered | null
    filter: string
}) {
    const childNodes = filtered
        ? node.children.filter((child) => filtered.visible.has(child.id))
        : node.children
    const hasChildren = childNodes.length > 0
    const isExpanded = hasChildren && (expanded.has(node.id) || Boolean(filtered?.autoExpand.has(node.id)))
    const paddingLeft = 8 + depth * 16

    return (
        <li>
            <div
                className="group flex items-center gap-1.5 pr-2 hover:bg-muted/60"
                style={{ paddingLeft }}
            >
                {hasChildren ? (
                    <button
                        type="button"
                        onClick={() => onToggle(node.id)}
                        aria-expanded={isExpanded}
                        aria-label={isExpanded ? `Colapsar ${node.name}` : `Expandir ${node.name}`}
                        className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:text-foreground"
                    >
                        <ChevronRightIcon className={cn("size-4 transition-transform", isExpanded && "rotate-90")} />
                    </button>
                ) : (
                    <span className="flex size-5 shrink-0 items-center justify-center">
                        <span className="size-1 rounded-full bg-muted-foreground/50" />
                    </span>
                )}

                <Link
                    to={`/settings/marketplace-categories/${node.id}`}
                    className="min-w-0 flex-1 truncate py-1.5 text-foreground underline-offset-4 hover:underline"
                >
                    {highlight(node.name, filter)}
                </Link>

                {node.children.length > 0 ? (
                    <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                        {node.children.length}
                    </span>
                ) : null}
            </div>

            {isExpanded ? (
                <ul>
                    {childNodes.map((child) => (
                        <TreeNode
                            key={child.id}
                            node={child}
                            depth={depth + 1}
                            expanded={expanded}
                            onToggle={onToggle}
                            filtered={filtered}
                            filter={filter}
                        />
                    ))}
                </ul>
            ) : null}
        </li>
    )
}

import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { categoriesResponseSchema, type CategoryTree } from "@/lib/schemas/categories";
import { useCategoriesPaginationStore } from "@/stores/categoriesPaginationStore";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

type FlatCategory = {
    id: number
    name: string
    parentName: string
}

function flattenCategories(tree: CategoryTree[]): FlatCategory[] {
    const rows: FlatCategory[] = []

    const walk = (nodes: CategoryTree[], parentName: string) => {
        for (const node of nodes) {
            rows.push({ id: node.id, name: node.name, parentName })
            if (node.children.length > 0) {
                walk(node.children, node.name)
            }
        }
    }

    walk(tree, "—")
    return rows
}

export function CategoriesPage() {
    const offset = useCategoriesPaginationStore((state) => state.offset)
    const pageSize = useCategoriesPaginationStore((state) => state.pageSize)
    const setOffset = useCategoriesPaginationStore((state) => state.setOffset)
    const setPageSize = useCategoriesPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["categories"],
        queryFn: async () => {
            const response = await apiClient.get("/api/categories")
            return categoriesResponseSchema.parse(response.data)
        }
    })

    const allCategories = useMemo(
        () => flattenCategories(data?.categories ?? []),
        [data]
    )

    const total = allCategories.length
    const categories = allCategories.slice(offset, offset + pageSize)

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Categorías</h2>
            <p className="mt-2 text-sm text-muted-foreground">Jerarquía de categorías del catálogo.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las categorías: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Categoría padre</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && categories.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={3} className="text-center text-muted-foreground">
                                Cargando categorías...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && categories.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={3} className="text-center text-muted-foreground">
                                No se encontraron categorías.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {categories.map((category) => (
                        <TableRow key={category.id}>
                            <TableCell>{category.name}</TableCell>
                            <TableCell>{category.parentName}</TableCell>
                            <TableCell></TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={total}
                onOffsetChange={setOffset}
                onPageSizeChange={setPageSize}
            />
        </section>
    )
}

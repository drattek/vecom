import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedBranchesResponseSchema } from "@/lib/schemas/branches";
import { useBranchesPaginationStore } from "@/stores/branchesPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

export function BranchesPage() {
    const offset = useBranchesPaginationStore((state) => state.offset)
    const pageSize = useBranchesPaginationStore((state) => state.pageSize)
    const setOffset = useBranchesPaginationStore((state) => state.setOffset)
    const setPageSize = useBranchesPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["branches", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/branches", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedBranchesResponseSchema.parse(response.data);
        }
    })

    const branches = data?.branches ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Sucursales</h2>
            <p className="mt-2 text-sm text-muted-foreground">Gestiona la estructura de sucursales y su disponibilidad operativa.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las sucursales: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Fecha de creación</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && branches.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                Cargando sucursales...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && branches.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                No se encontraron sucursales.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {branches.map((branch) => (
                        <TableRow key={branch.id}>
                            <TableCell>{branch.name}</TableCell>
                            <TableCell>{formatDate(branch.createdAt)}</TableCell>
                            <TableCell>{formatDate(branch.updatedAt)}</TableCell>
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

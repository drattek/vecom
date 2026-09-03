import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedWarehousesResponseSchema } from "@/lib/schemas/warehouses";
import { useWarehousesPaginationStore } from "@/stores/warehousesPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

export function WarehousesPage() {
    const offset = useWarehousesPaginationStore((state) => state.offset)
    const pageSize = useWarehousesPaginationStore((state) => state.pageSize)
    const setOffset = useWarehousesPaginationStore((state) => state.setOffset)
    const setPageSize = useWarehousesPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["warehouses", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/warehouses", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedWarehousesResponseSchema.parse(response.data);
        }
    })

    const warehouses = data?.warehouses ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Almacenes</h2>
            <p className="mt-2 text-sm text-muted-foreground">Define almacenes, capacidad y asignaciones para inventario.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los almacenes: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Sucursal</TableHead>
                        <TableHead>Fecha de creación</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && warehouses.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                Cargando almacenes...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && warehouses.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                No se encontraron almacenes.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {warehouses.map((warehouse) => (
                        <TableRow key={warehouse.id}>
                            <TableCell>{warehouse.name}</TableCell>
                            <TableCell>{warehouse.branchId}</TableCell>
                            <TableCell>{formatDate(warehouse.createdAt)}</TableCell>
                            <TableCell>{formatDate(warehouse.updatedAt)}</TableCell>
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

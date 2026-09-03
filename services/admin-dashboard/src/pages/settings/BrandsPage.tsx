import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedBrandsResponseSchema } from "@/lib/schemas/brands";
import { useBrandsPaginationStore } from "@/stores/brandsPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

export function BrandsPage() {
    const offset = useBrandsPaginationStore((state) => state.offset)
    const pageSize = useBrandsPaginationStore((state) => state.pageSize)
    const setOffset = useBrandsPaginationStore((state) => state.setOffset)
    const setPageSize = useBrandsPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["brands", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/brands", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedBrandsResponseSchema.parse(response.data);
        }
    })

    const brands = data?.brands ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Marcas</h2>
            <p className="mt-2 text-sm text-muted-foreground">Marcas disponibles en el catálogo de productos.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las marcas: {error instanceof Error ? error.message : "Error desconocido"}
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
                    {isLoading && brands.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                Cargando marcas...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && brands.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                No se encontraron marcas.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {brands.map((brand) => (
                        <TableRow key={brand.id}>
                            <TableCell>{brand.name}</TableCell>
                            <TableCell>{formatDate(brand.createdAt)}</TableCell>
                            <TableCell>{formatDate(brand.updatedAt)}</TableCell>
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

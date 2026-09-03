import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedStorageDisksResponseSchema } from "@/lib/schemas/storage-disk";
import { useStoragePaginationStore } from "@/stores/storageDisckPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

export function StorageDiskPage() {
    const offset = useStoragePaginationStore((state) => state.offset)
    const pageSize = useStoragePaginationStore((state) => state.pageSize)
    const setOffset = useStoragePaginationStore((state) => state.setOffset)
    const setPageSize = useStoragePaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["storage-disk", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/storage-disks", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedStorageDisksResponseSchema.parse(response.data);
        }
    })

    const storageDisks = data?.storageDisks ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Discos de almacenamiento</h2>
            <p className="mt-2 text-sm text-muted-foreground">Administra discos y almacenamiento físico usado por la plataforma.</p>
        
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los discos de almacenamiento: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Código</TableHead>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Contenedor</TableHead>
                        <TableHead>Público</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && storageDisks.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                Cargando almacenamientos...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && storageDisks.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                No se encontraron almacenamientos.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {storageDisks.map((storageDisk) => (
                        <TableRow key={storageDisk.id}>
                            <TableCell>{storageDisk.code}</TableCell>
                            <TableCell>{storageDisk.name}</TableCell>
                            <TableCell>{storageDisk.bucket}</TableCell>
                            <TableCell>{storageDisk.isPublic ? "Si" : "No"}</TableCell>
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
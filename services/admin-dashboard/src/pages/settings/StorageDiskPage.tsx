import { Field, FieldLabel } from "@/components/ui/field";
import { Pagination, PaginationContent, PaginationItem, PaginationLink, PaginationNext, PaginationPrevious } from "@/components/ui/pagination";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedStorageDisksResponseSchema } from "@/lib/schemas/storage-disk";
import { useStoragePaginationStore } from "@/stores/storageDisckPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import type { MouseEvent } from "react";

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
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    const currentPage = Math.min(totalPages, Math.floor(offset / pageSize) + 1)
    const canGoPrevious = offset > 0
    const canGoNext = offset + pageSize < total

    const handlePageSizeChange = (value: string | null) => {
        if (!value) {
            return
        }

        const parsed = parseInt(value, 10)
        if (Number.isNaN(parsed) || parsed <= 0) {
            return
        }
        setPageSize(parsed)
    }

    const goToPreviousPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        if (!canGoPrevious) {
            return
        }

        setOffset(Math.max(0, offset - pageSize))
    }

    const goToNextPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        if (!canGoNext) {
            return
        }

        setOffset(offset + pageSize)
    }

    return (
        <section className="p-4">
            <h2 className="text-xl font-semibold text-foreground">Discos de almacenamiento</h2>
            <p className="mt-2 text-sm text-muted-foreground">Administra discos y almacenamiento físico usado por la plataforma.</p>
        
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los discos de almacenamiento: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <div className="flex items-center justify-end">
                <div className="flex-1"></div>
                <div className="flex items-center gap-2">
                    <Field orientation="horizontal">
                        <FieldLabel htmlFor="rows-per-page">Cantidad:</FieldLabel>
                        <Select value={pageSize.toString()} onValueChange={handlePageSizeChange}>
                            <SelectTrigger id="rows-per-page">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent align="start">
                                <SelectGroup>
                                    <SelectItem value="10">10</SelectItem>
                                    <SelectItem value="25">25</SelectItem>
                                    <SelectItem value="50">50</SelectItem>
                                </SelectGroup>
                            </SelectContent>
                        </Select>
                    </Field>
                    <Pagination>
                        <PaginationContent>
                            <PaginationItem>
                                <PaginationPrevious 
                                    href="#"
                                    onClick={goToPreviousPage}
                                    aria-disabled={!canGoPrevious}
                                    className={!canGoPrevious ? "pointer-events-none opacity-50" : ""}
                                >
                                    Previous
                                </PaginationPrevious>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationLink href="#">1</PaginationLink>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationNext 
                                    href="#"
                                    onClick={goToNextPage}
                                    aria-disabled={!canGoNext}
                                    className={!canGoNext ? "pointer-events-none opacity-50" : ""}
                                >
                                    Next
                                </PaginationNext>
                            </PaginationItem>
                        </PaginationContent>
                    </Pagination>
                </div>
            </div>

            <Table>
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
        </section>
    )
}
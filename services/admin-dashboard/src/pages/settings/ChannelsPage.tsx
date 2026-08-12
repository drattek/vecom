import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Field, FieldLabel } from "@/components/ui/field";
import { Pagination, PaginationContent, PaginationItem, PaginationLink, PaginationNext, PaginationPrevious } from "@/components/ui/pagination";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedChannelsResponseSchema } from "@/lib/schemas/channels";
import { useChannelsPaginationStore } from "@/stores/channelsPaginationStore";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { MouseEvent } from "react";
import type { Channel } from "@/lib/schemas/channels";

function statusLabel(status: Channel["status"]) {
    if (status === "active") return "Activo";
    if (status === "discontinued") return "Descontinuado";
    return "Oculto";
}

export function ChannelsPage() {
    const offset = useChannelsPaginationStore((state) => state.offset);
    const pageSize = useChannelsPaginationStore((state) => state.pageSize);
    const setOffset = useChannelsPaginationStore((state) => state.setOffset);
    const setPageSize = useChannelsPaginationStore((state) => state.setPageSize);

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["channels", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/channels", {
                params: {
                    offset,
                    pageSize,
                },
            });

            return paginatedChannelsResponseSchema.parse(response.data);
        },
    });

    const channels = data?.channels ?? [];
    const total = data?.total ?? 0;
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    const currentPage = Math.min(totalPages, Math.floor(offset / pageSize) + 1);
    const canGoPrevious = offset > 0;
    const canGoNext = offset + pageSize < total;

    const handlePageSizeChange = (value: string | null) => {
        if (!value) {
            return;
        }

        const parsed = Number.parseInt(value, 10);
        if (Number.isNaN(parsed) || parsed <= 0) {
            return;
        }

        setPageSize(parsed);
    };

    const goToPreviousPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault();
        if (!canGoPrevious) {
            return;
        }

        setOffset(Math.max(0, offset - pageSize));
    };

    const goToNextPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault();
        if (!canGoNext) {
            return;
        }

        setOffset(offset + pageSize);
    };

    return (
        <section className="p-4">
            <h2 className="text-xl font-semibold text-foreground">Canales</h2>
            <p className="mt-2 text-sm text-muted-foreground">Configura canales de venta por marketplace y su disponibilidad.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al cargar canales: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <div className="flex items-center justify-end">
                <div className="flex-1"></div>
                <div className="flex items-center gap-2">
                    <Field orientation="horizontal">
                        <FieldLabel htmlFor="rows-per-page">Cantidad:</FieldLabel>
                        <Select value={String(pageSize)} onValueChange={handlePageSizeChange}>
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
                                    className={!canGoPrevious ? "pointer-events-none opacity-50" : undefined}
                                >
                                    Previous
                                </PaginationPrevious>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationLink href="#" isActive onClick={(event) => event.preventDefault()}>
                                    {currentPage}
                                </PaginationLink>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationNext
                                    href="#"
                                    onClick={goToNextPage}
                                    aria-disabled={!canGoNext}
                                    className={!canGoNext ? "pointer-events-none opacity-50" : undefined}
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
                        <TableHead>Logo</TableHead>
                        <TableHead>Código</TableHead>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Conexiones</TableHead>
                        <TableHead>Estado</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && channels.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                Cargando canales...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && channels.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                No hay canales para mostrar.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {channels.map((channel) => (
                        <TableRow key={channel.id}>
                            <TableCell>
                                <div className="flex size-8 items-center justify-center rounded-md border border-border bg-muted text-xs font-semibold uppercase text-muted-foreground">
                                    {channel.name.slice(0, 2)}
                                </div>
                            </TableCell>
                            <TableCell>{channel.code}</TableCell>
                            <TableCell>{channel.name}</TableCell>
                            <TableCell>{channel.connectionCount}</TableCell>
                            <TableCell>{statusLabel(channel.status)}</TableCell>
                            <TableCell>
                                <DropdownMenu>
                                    <DropdownMenuTrigger
                                        render={
                                            <Button variant="outline">Open</Button>
                                        }
                                    />
                                    <DropdownMenuContent>
                                        <DropdownMenuGroup>
                                            <DropdownMenuItem>Editar</DropdownMenuItem>
                                        </DropdownMenuGroup>
                                    </DropdownMenuContent>
                                </DropdownMenu>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </section>
    )
}
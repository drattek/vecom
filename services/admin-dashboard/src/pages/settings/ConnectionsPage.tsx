import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedChannelConnectionsResponseSchema } from "@/lib/schemas/channel-connections";
import { useChannelConnectionsPaginationStore } from "@/stores/channelConnectionsPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

function statusLabel(status: string): string {
    if (status === "active") return "Activo"
    if (status === "discontinued") return "Descontinuado"
    if (status === "hidden") return "Oculto"
    return status || "-"
}

function environmentLabel(environment: string): string {
    if (environment === "production") return "Producción"
    if (environment === "development") return "Desarrollo"
    return environment || "-"
}

export function ConnectionsPage() {
    const offset = useChannelConnectionsPaginationStore((state) => state.offset)
    const pageSize = useChannelConnectionsPaginationStore((state) => state.pageSize)
    const setOffset = useChannelConnectionsPaginationStore((state) => state.setOffset)
    const setPageSize = useChannelConnectionsPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["channel-connections", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-connections", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedChannelConnectionsResponseSchema.parse(response.data);
        }
    })

    const connections = data?.channelConnections ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Conexiones</h2>
            <p className="mt-2 text-sm text-muted-foreground">Administra credenciales y estados de conexión con marketplaces.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las conexiones: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Canal</TableHead>
                        <TableHead>Estado</TableHead>
                        <TableHead>Ambiente</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && connections.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                Cargando conexiones...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && connections.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                No se encontraron conexiones.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {connections.map((connection) => (
                        <TableRow key={connection.id}>
                            <TableCell>{connection.name}</TableCell>
                            <TableCell>{connection.channelName || `#${connection.channelId}`}</TableCell>
                            <TableCell>{statusLabel(connection.status)}</TableCell>
                            <TableCell>{environmentLabel(connection.environment)}</TableCell>
                            <TableCell>{formatDate(connection.updatedAt)}</TableCell>
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

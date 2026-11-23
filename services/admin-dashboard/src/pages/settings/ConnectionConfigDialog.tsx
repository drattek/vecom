import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { apiClient } from "@/lib/api"
import type { ChannelConnection } from "@/lib/schemas/channel-connections"
import {
    connectionConfigSchema,
    type ConnectionConfigEntry,
} from "@/lib/schemas/connection-config"
import { useQuery } from "@tanstack/react-query"

interface ConnectionConfigDialogProps {
    connection: ChannelConnection | null
    open: boolean
    onOpenChange: (open: boolean) => void
}

function formatDate(value: string | null | undefined): string {
    if (!value) return "-"
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) return "-"
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

function EntriesTable({ title, emptyText, entries }: { title: string; emptyText: string; entries: ConnectionConfigEntry[] }) {
    return (
        <div>
            <h3 className="text-sm font-medium">{title}</h3>
            <Table containerClassName="mt-2 max-h-56 overflow-auto rounded-lg border border-border">
                <TableHeader>
                    <TableRow>
                        <TableHead>Clave</TableHead>
                        <TableHead>Valor</TableHead>
                        <TableHead>Actualizado</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {entries.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={3} className="text-center text-muted-foreground">
                                {emptyText}
                            </TableCell>
                        </TableRow>
                    ) : null}
                    {entries.map((entry) => (
                        <TableRow key={entry.keyName}>
                            <TableCell className="font-mono text-xs">{entry.keyName}</TableCell>
                            <TableCell className="max-w-[18rem] break-all font-mono text-xs">
                                {entry.value === "" ? <span className="text-muted-foreground">(vacío)</span> : entry.value}
                            </TableCell>
                            <TableCell className="text-xs text-muted-foreground">{formatDate(entry.updatedAt)}</TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    )
}

// ConnectionConfigDialog muestra en solo lectura lo que una conexión tiene
// configurado. Las credenciales siempre llegan enmascaradas desde el backend.
export function ConnectionConfigDialog({ connection, open, onOpenChange }: ConnectionConfigDialogProps) {
    const connectionId = connection?.id

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["channel-connection-config", connectionId],
        enabled: open && connectionId !== undefined,
        queryFn: async () => {
            const response = await apiClient.get(`/api/channel-connections/${connectionId}/config`)
            return connectionConfigSchema.parse(response.data)
        },
    })

    if (!connection) return null

    const status = data?.status

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>Configuración de la conexión</DialogTitle>
                    <DialogDescription>
                        {connection.name} · {connection.channelName || `Canal #${connection.channelId}`}. Solo lectura;
                        las credenciales y los valores cifrados se muestran ocultos.
                    </DialogDescription>
                </DialogHeader>

                {isLoading ? <p className="text-sm text-muted-foreground">Cargando configuración...</p> : null}
                {isError ? (
                    <p className="text-sm text-destructive">
                        No se pudo cargar la configuración: {error instanceof Error ? error.message : "error desconocido"}
                    </p>
                ) : null}

                {data ? (
                    <div className="flex max-h-[60vh] flex-col gap-4 overflow-auto pr-1">
                        <div>
                            <h3 className="text-sm font-medium">Estado</h3>
                            {status ? (
                                <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
                                    <dt className="text-muted-foreground">Autenticada</dt>
                                    <dd>{status.authenticated ? "Sí" : "No"}</dd>
                                    <dt className="text-muted-foreground">Última autenticación</dt>
                                    <dd>{formatDate(status.lastAuth)}</dd>
                                    <dt className="text-muted-foreground">El token expira</dt>
                                    <dd>{formatDate(status.expiresAt)}</dd>
                                    <dt className="text-muted-foreground">Última sincronización</dt>
                                    <dd>{formatDate(status.lastSync)}</dd>
                                    {status.lastError ? (
                                        <>
                                            <dt className="text-muted-foreground">Último error</dt>
                                            <dd className="break-words text-destructive">{status.lastError}</dd>
                                        </>
                                    ) : null}
                                </dl>
                            ) : (
                                <p className="mt-2 text-sm text-muted-foreground">Esta conexión aún no tiene estado registrado.</p>
                            )}
                        </div>

                        <EntriesTable title="Credenciales" emptyText="Sin credenciales configuradas." entries={data.credentials} />
                        <EntriesTable title="Ajustes" emptyText="Sin ajustes configurados." entries={data.settings} />
                    </div>
                ) : null}

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cerrar
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

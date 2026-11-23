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
import type { Channel } from "@/lib/schemas/channels"
import { channelParametersResponseSchema } from "@/lib/schemas/connection-config"
import { useQuery } from "@tanstack/react-query"

interface ChannelConfigDialogProps {
    channel: Channel | null
    open: boolean
    onOpenChange: (open: boolean) => void
}

const MASKED = "••••••••"

function statusLabel(status: Channel["status"]) {
    if (status === "active") return "Activo"
    if (status === "discontinued") return "Descontinuado"
    return "Oculto"
}

function typeLabel(type: string) {
    if (type === "text") return "Texto"
    if (type === "number") return "Número"
    if (type === "boolean") return "Booleano"
    return type || "-"
}

// ChannelConfigDialog muestra en solo lectura los parámetros que un canal
// exige configurar a cada una de sus conexiones.
export function ChannelConfigDialog({ channel, open, onOpenChange }: ChannelConfigDialogProps) {
    const channelId = channel?.id

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["channel-parameters", channelId],
        enabled: open && channelId !== undefined,
        queryFn: async () => {
            const response = await apiClient.get(`/api/channels/${channelId}/parameters`, {
                params: { offset: 0, pageSize: 100 },
            })
            return channelParametersResponseSchema.parse(response.data)
        },
    })

    if (!channel) return null

    const parameters = data?.data ?? []

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>Configuración del canal</DialogTitle>
                    <DialogDescription>
                        {channel.name} ({channel.code}) · {statusLabel(channel.status)} · {channel.connectionCount}{" "}
                        {channel.connectionCount === 1 ? "conexión" : "conexiones"}. Solo lectura.
                    </DialogDescription>
                </DialogHeader>

                {channel.description ? <p className="text-sm text-muted-foreground">{channel.description}</p> : null}

                {isLoading ? <p className="text-sm text-muted-foreground">Cargando parámetros...</p> : null}
                {isError ? (
                    <p className="text-sm text-destructive">
                        No se pudieron cargar los parámetros: {error instanceof Error ? error.message : "error desconocido"}
                    </p>
                ) : null}

                {data ? (
                    <div>
                        <h3 className="text-sm font-medium">Parámetros ({data.total})</h3>
                        <Table containerClassName="mt-2 max-h-72 overflow-auto rounded-lg border border-border">
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Nombre</TableHead>
                                    <TableHead>Clave</TableHead>
                                    <TableHead>Tipo</TableHead>
                                    <TableHead>Requerido</TableHead>
                                    <TableHead>Valor por defecto</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {parameters.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={5} className="text-center text-muted-foreground">
                                            Este canal no tiene parámetros configurados.
                                        </TableCell>
                                    </TableRow>
                                ) : null}
                                {parameters.map((parameter) => (
                                    <TableRow key={parameter.id}>
                                        <TableCell>{parameter.displayName}</TableCell>
                                        <TableCell className="font-mono text-xs">{parameter.parameterName}</TableCell>
                                        <TableCell>{typeLabel(parameter.parameterType)}</TableCell>
                                        <TableCell>{parameter.required ? "Sí" : "No"}</TableCell>
                                        <TableCell className="max-w-[14rem] break-all font-mono text-xs">
                                            {parameter.defaultValue
                                                ? parameter.isEncrypted
                                                    ? MASKED
                                                    : parameter.defaultValue
                                                : <span className="text-muted-foreground">-</span>}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
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

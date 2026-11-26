import { RefreshCwIcon } from "lucide-react"

import { Can } from "@/auth/Can"
import { Button } from "@/components/ui/button"
import { getServerErrorMessage } from "@/lib/api"
import { formatDateTime } from "@/lib/dashboard"
import { useRequestSalesSync } from "@/lib/sales"
import type { SalesSyncConnection } from "@/lib/schemas/sales"

type SalesSyncPanelProps = {
    enabled: boolean
    intervalMinutes: number
    connections: SalesSyncConnection[]
}

function formatInterval(minutes: number): string {
    return minutes % 60 === 0 ? `${minutes / 60} h` : `${minutes} min`
}

/** Última sincronización y errores de cada conexión. */
export function SalesSyncPanel({ enabled, intervalMinutes, connections }: SalesSyncPanelProps) {
    const sync = useRequestSalesSync()
    const active = connections.filter((c) => c.active)
    const failing = active.filter((c) => c.lastError).length

    return (
        <div className="mt-3 shrink-0 rounded-lg border border-border p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-xs text-muted-foreground">
                    {enabled
                        ? `Se actualiza cada ${formatInterval(intervalMinutes)}; un cambio puede tardar hasta ese tiempo en verse.`
                        : "La actualización automática está apagada en el servidor."}
                </p>
                <Can any={["sales.sync"]}>
                    <Button
                        variant="outline"
                        size="sm"
                        disabled={sync.isPending || active.every((c) => c.running)}
                        onClick={() => sync.mutate(undefined)}
                    >
                        <RefreshCwIcon /> Sincronizar todas
                    </Button>
                </Can>
            </div>

            {sync.isError ? (
                <p className="mt-2 text-xs text-destructive" role="alert">
                    {getServerErrorMessage(sync.error, "No se pudo iniciar la sincronización.")}
                </p>
            ) : null}

            {active.length === 0 ? (
                <p className="mt-2 text-sm text-muted-foreground">
                    No hay conexiones activas de MercadoLibre u Odoo.
                </p>
            ) : (
                <details className="mt-2">
                    <summary className="cursor-pointer text-sm font-medium">
                        Estado por conexión ({active.length})
                        {failing > 0 ? <span className="ml-2 text-destructive">{failing} con errores</span> : null}
                        {active.some((c) => c.running) ? (
                            <span className="ml-2 text-muted-foreground">Sincronizando…</span>
                        ) : null}
                    </summary>
                    <ul className="mt-2 max-h-48 space-y-2 overflow-auto">
                        {active.map((connection) => (
                            <li key={connection.connectionId} className="flex items-start justify-between gap-3 text-sm">
                                <div className="min-w-0">
                                    <p>
                                        {connection.connectionName}{" "}
                                        <span className="text-xs text-muted-foreground">({connection.channelName})</span>
                                    </p>
                                    <p className="text-xs text-muted-foreground">
                                        {connection.running
                                            ? "Sincronizando…"
                                            : connection.lastSuccessAt
                                              ? `Última correcta: ${formatDateTime(connection.lastSuccessAt)}`
                                              : "Aún no completa su primera carga"}
                                        {connection.lastFinishedAt && !connection.running
                                            ? ` · Última corrida: ${connection.lastSavedCount} guardadas, ${connection.lastFailedCount} con error`
                                            : ""}
                                    </p>
                                    {connection.lastError ? (
                                        <p className="mt-0.5 text-xs break-words text-destructive">{connection.lastError}</p>
                                    ) : null}
                                </div>
                                <Can any={["sales.sync"]}>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        disabled={sync.isPending || connection.running}
                                        onClick={() => sync.mutate(connection.connectionId)}
                                    >
                                        Sincronizar ahora
                                    </Button>
                                </Can>
                            </li>
                        ))}
                    </ul>
                </details>
            )}
        </div>
    )
}

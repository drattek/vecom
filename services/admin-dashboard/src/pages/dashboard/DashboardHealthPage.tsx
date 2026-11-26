import type { ReactNode } from "react"

import { BlockError, DashboardTable } from "@/components/dashboard/DashboardTable"
import { TableCell, TableRow } from "@/components/ui/table"
import { formatDateTime, useConnectionsHealth, useInventoryHealth, useQueueHealth } from "@/lib/dashboard"
import type { ConnectionHealth } from "@/lib/schemas/dashboard"

const SYNC_TYPE_LABELS: Record<string, string> = {
    price: "Precio",
    stock: "Stock",
    full: "Completa",
    listing: "Publicación",
    compatibilities: "Compatibilidades",
    resync: "Resincronización",
}

const OK_TONE = "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
const WARN_TONE = "bg-amber-500/10 text-amber-700 dark:text-amber-400"
const BAD_TONE = "bg-destructive/10 text-destructive"
const NEUTRAL_TONE = "bg-muted text-muted-foreground"

function connectionBadge(connection: ConnectionHealth): { label: string; tone: string } {
    if (connection.status !== "active") return { label: "Inactiva", tone: NEUTRAL_TONE }
    if (!connection.hasStatus) return { label: "Sin registro", tone: NEUTRAL_TONE }
    if (!connection.authenticated) return { label: "Sin autenticar", tone: BAD_TONE }
    if (connection.expiresAt && new Date(connection.expiresAt).getTime() <= Date.now()) {
        return { label: "Token vencido", tone: BAD_TONE }
    }
    if (connection.lastError) return { label: "Con error reciente", tone: WARN_TONE }
    return { label: "Autenticada", tone: OK_TONE }
}

function Block({ title, description, children }: { title: string; description: string; children: ReactNode }) {
    return (
        <section className="rounded-xl border bg-card p-4">
            <h3 className="font-semibold text-foreground">{title}</h3>
            <p className="mb-3 text-sm text-muted-foreground">{description}</p>
            {children}
        </section>
    )
}

function ConnectionsBlock() {
    const query = useConnectionsHealth()
    const connections = query.data ?? []

    return (
        <Block title="Conexiones" description="Último estado de autenticación registrado por conexión.">
            {query.isError ? (
                <BlockError error={query.error} onRetry={() => void query.refetch()} />
            ) : (
                <DashboardTable
                    columns={["Conexión", "Estado", "Última autenticación", "Expira", "Último error"]}
                    isLoading={query.isPending}
                    isEmpty={connections.length === 0}
                    emptyMessage="No hay conexiones configuradas."
                >
                    {connections.map((connection) => {
                        const badge = connectionBadge(connection)
                        return (
                            <TableRow key={connection.connectionId}>
                                <TableCell>
                                    <div className="font-medium">{connection.connectionName}</div>
                                    <div className="text-xs text-muted-foreground">
                                        {connection.channelName} ·{" "}
                                        {connection.environment === "production" ? "Producción" : "Desarrollo"}
                                    </div>
                                </TableCell>
                                <TableCell>
                                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${badge.tone}`}>
                                        {badge.label}
                                    </span>
                                </TableCell>
                                <TableCell>{formatDateTime(connection.lastAuth)}</TableCell>
                                <TableCell>{formatDateTime(connection.expiresAt)}</TableCell>
                                <TableCell className="min-w-64 whitespace-normal">
                                    <div className="line-clamp-3 text-xs break-words" title={connection.lastError ?? undefined}>
                                        {connection.lastError ?? "—"}
                                    </div>
                                </TableCell>
                            </TableRow>
                        )
                    })}
                </DashboardTable>
            )}
        </Block>
    )
}

function InventoryBlock() {
    const query = useInventoryHealth()
    const sources = query.data ?? []

    return (
        <Block
            title="Inventario"
            description="Última vez que una sincronización cambió una cantidad, por fuente. Una corrida sin cambios no actualiza esta fecha."
        >
            {query.isError ? (
                <BlockError error={query.error} onRetry={() => void query.refetch()} />
            ) : (
                <DashboardTable
                    columns={["Fuente", "Último cambio de inventario", "Registros de stock"]}
                    isLoading={query.isPending}
                    isEmpty={sources.length === 0}
                    emptyMessage="No hay registros de inventario."
                >
                    {sources.map((source) => (
                        <TableRow key={source.sourceId}>
                            <TableCell className="font-medium">{source.sourceName}</TableCell>
                            <TableCell>{formatDateTime(source.lastChangeAt)}</TableCell>
                            <TableCell className="tabular-nums">{source.stockRows.toLocaleString("es-MX")}</TableCell>
                        </TableRow>
                    ))}
                </DashboardTable>
            )}
        </Block>
    )
}

function QueueBlock() {
    const query = useQueueHealth()
    const queues = query.data ?? []

    return (
        <Block title="Cola de sincronización" description="Trabajos pendientes, en proceso o fallidos.">
            {query.isError ? (
                <BlockError error={query.error} onRetry={() => void query.refetch()} />
            ) : (
                <DashboardTable
                    columns={["Conexión", "Tipo", "Pendientes", "En proceso", "Fallidos", "Pendiente más antiguo"]}
                    isLoading={query.isPending}
                    isEmpty={queues.length === 0}
                    emptyMessage="La cola no tiene trabajos pendientes ni fallidos."
                >
                    {queues.map((queue) => (
                        <TableRow key={`${queue.connectionId}-${queue.syncType}`}>
                            <TableCell className="font-medium">{queue.connectionName}</TableCell>
                            <TableCell>{SYNC_TYPE_LABELS[queue.syncType] ?? queue.syncType}</TableCell>
                            <TableCell className="tabular-nums">{queue.pending.toLocaleString("es-MX")}</TableCell>
                            <TableCell className="tabular-nums">{queue.processing.toLocaleString("es-MX")}</TableCell>
                            <TableCell className={`tabular-nums ${queue.failed > 0 ? "font-medium text-destructive" : ""}`}>
                                {queue.failed.toLocaleString("es-MX")}
                            </TableCell>
                            <TableCell>{formatDateTime(queue.oldestPendingAt)}</TableCell>
                        </TableRow>
                    ))}
                </DashboardTable>
            )}
        </Block>
    )
}

export function DashboardHealthPage() {
    return (
        <>
            <h2 className="text-xl font-semibold text-foreground">Salud de la plataforma</h2>
            <p className="mt-1 text-sm text-muted-foreground">Conexiones, inventario y trabajos en segundo plano.</p>
            <div className="mt-4 flex flex-col gap-4">
                <ConnectionsBlock />
                <InventoryBlock />
                <QueueBlock />
            </div>
        </>
    )
}

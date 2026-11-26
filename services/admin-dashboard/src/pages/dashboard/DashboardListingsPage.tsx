import { useState } from "react"

import { BlockError, DashboardTable } from "@/components/dashboard/DashboardTable"
import { ProductCell } from "@/components/dashboard/ProductCell"
import { TablePagination } from "@/components/TablePagination"
import { Skeleton } from "@/components/ui/skeleton"
import { TableCell, TableRow } from "@/components/ui/table"
import { formatDateTime, useAttentionPage } from "@/lib/dashboard"
import type { AttentionState } from "@/lib/schemas/dashboard"

const DEFAULT_PAGE_SIZE = 10

type StateConfig = {
    state: AttentionState
    title: string
    description: string
    tone: string
}

// En orden de urgencia.
const STATES: StateConfig[] = [
    {
        state: "error",
        title: "Con error",
        description: "Publicaciones existentes cuya última operación quedó en error.",
        tone: "text-destructive",
    },
    {
        state: "not_published",
        title: "No publicadas",
        description: "El último intento de publicar falló y no existe una publicación activa.",
        tone: "text-destructive",
    },
    {
        state: "under_review",
        title: "En revisión",
        description: "El marketplace las tiene en revisión.",
        tone: "text-sky-700 dark:text-sky-400",
    },
    {
        state: "paused_manual",
        title: "Pausadas",
        description: "Pausadas por un motivo distinto a la falta de stock.",
        tone: "text-orange-700 dark:text-orange-400",
    },
    {
        state: "paused_zero_stock",
        title: "Pausadas sin stock",
        description: "El middleware las pausó porque no hay existencias.",
        tone: "text-amber-700 dark:text-amber-400",
    },
]

const COLUMNS = ["Producto", "Conexión", "Publicación", "Detalle", "Última actividad"]

const sectionId = (state: AttentionState) => `dashboard-listings-${state}`

/** Contador de un estado; comparte consulta con la primera página de su tabla. */
function StateCard({ config }: { config: StateConfig }) {
    const query = useAttentionPage(config.state, 0, DEFAULT_PAGE_SIZE)

    return (
        <button
            type="button"
            onClick={() => document.getElementById(sectionId(config.state))?.scrollIntoView({ behavior: "smooth" })}
            className="rounded-lg border border-border p-3 text-left outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50"
        >
            <span className="block text-xs text-muted-foreground">{config.title}</span>
            {query.isPending ? (
                <Skeleton className="mt-1 h-8 w-16" />
            ) : (
                <span className={`block text-2xl font-semibold tabular-nums ${config.tone}`}>
                    {query.data ? query.data.total.toLocaleString("es-MX") : "—"}
                </span>
            )}
        </button>
    )
}

function StateSection({ config }: { config: StateConfig }) {
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
    const query = useAttentionPage(config.state, offset, pageSize)
    const rows = query.data?.rows ?? []
    const total = query.data?.total ?? 0

    return (
        <section id={sectionId(config.state)} className="scroll-mt-2 rounded-xl border bg-card p-4">
            <h3 className={`font-semibold ${config.tone}`}>{config.title}</h3>
            <p className="mb-3 text-sm text-muted-foreground">{config.description}</p>

            {query.isError ? (
                <BlockError error={query.error} onRetry={() => void query.refetch()} />
            ) : (
                <>
                    <DashboardTable
                        columns={COLUMNS}
                        isLoading={query.isPending}
                        isStale={query.isPlaceholderData}
                        isEmpty={rows.length === 0}
                        emptyMessage="No hay publicaciones en este estado."
                    >
                        {rows.map((row) => (
                            <TableRow key={`${row.kind}-${row.id}`}>
                                <TableCell>
                                    <ProductCell productId={row.productId} sku={row.sku} name={row.productName} />
                                </TableCell>
                                <TableCell>{row.connectionName}</TableCell>
                                <TableCell>
                                    <div className="max-w-64 truncate" title={row.listingTitle ?? undefined}>
                                        {row.listingTitle ?? "—"}
                                    </div>
                                    {row.externalId ? (
                                        <div className="font-mono text-xs text-muted-foreground">{row.externalId}</div>
                                    ) : null}
                                </TableCell>
                                <TableCell className="min-w-64 whitespace-normal">
                                    {row.lastError ? (
                                        <div className="line-clamp-3 text-xs break-words" title={row.lastError}>
                                            {row.lastError}
                                        </div>
                                    ) : (
                                        <span className="text-muted-foreground">—</span>
                                    )}
                                </TableCell>
                                <TableCell>{formatDateTime(row.lastActivityAt)}</TableCell>
                            </TableRow>
                        ))}
                    </DashboardTable>
                    {total > DEFAULT_PAGE_SIZE ? (
                        <TablePagination
                            offset={offset}
                            pageSize={pageSize}
                            total={total}
                            onOffsetChange={setOffset}
                            onPageSizeChange={(size) => {
                                setPageSize(size)
                                setOffset(0)
                            }}
                        />
                    ) : null}
                </>
            )}
        </section>
    )
}

export function DashboardListingsPage() {
    return (
        <>
            <h2 className="text-xl font-semibold text-foreground">Publicaciones que requieren atención</h2>
            <p className="mt-1 text-sm text-muted-foreground">
                Todos los canales, separadas por estado. Selecciona un contador para ir a su tabla.
            </p>

            <div className="mt-4 grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-5">
                {STATES.map((config) => (
                    <StateCard key={config.state} config={config} />
                ))}
            </div>

            <div className="mt-4 flex flex-col gap-4">
                {STATES.map((config) => (
                    <StateSection key={config.state} config={config} />
                ))}
            </div>
        </>
    )
}

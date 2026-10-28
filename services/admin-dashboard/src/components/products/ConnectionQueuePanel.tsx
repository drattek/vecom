import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useConnectionQueue, type QueueSummary } from "@/lib/channel-listings"
import { ListTodoIcon, Loader2Icon } from "lucide-react"
import { useState } from "react"

const PAGE_SIZE = 25

const SYNC_TYPE_LABELS: Record<string, string> = {
    resync: "Resincronización",
    compatibilities: "Compatibilidades",
    listing: "Publicación",
}

const STATUS_LABELS: Record<string, string> = {
    pending: "Pendiente",
    processing: "Procesando",
    done: "Listo",
    failed: "Con error",
}

const STATUS_FILTERS = ["", "pending", "processing", "done", "failed"] as const

function syncTypeLabel(syncType: string) {
    return SYNC_TYPE_LABELS[syncType] ?? syncType
}

function formatDate(value?: string | null) {
    return value ? new Date(value).toLocaleString() : "—"
}

function ProgressRow({ summary }: { summary: QueueSummary }) {
    const total = summary.pending + summary.processing + summary.done + summary.failed
    const finished = summary.done + summary.failed
    const percent = total === 0 ? 0 : Math.round((finished / total) * 100)
    const running = summary.pending + summary.processing > 0

    return (
        <div className="flex flex-col gap-1">
            <div className="flex items-center justify-between gap-4 text-sm">
                <span className="flex items-center gap-2 font-medium">
                    {running ? <Loader2Icon className="size-3.5 animate-spin" /> : null}
                    {syncTypeLabel(summary.syncType)}
                </span>
                <span className="text-muted-foreground">
                    {finished} de {total} ({percent}%)
                </span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                <div className="h-full bg-primary transition-all" style={{ width: `${percent}%` }} />
            </div>
            <div className="flex flex-wrap gap-x-4 text-xs text-muted-foreground">
                <span>Pendientes: {summary.pending}</span>
                <span>Procesando: {summary.processing}</span>
                <span>Listos: {summary.done}</span>
                <span className={summary.failed > 0 ? "text-destructive" : undefined}>Con error: {summary.failed}</span>
            </div>
        </div>
    )
}

function QueueDetailDialog({
    connectionId,
    syncTypes,
    open,
    onOpenChange,
}: {
    connectionId: number
    syncTypes: string[]
    open: boolean
    onOpenChange: (open: boolean) => void
}) {
    const [selectedType, setSelectedType] = useState<string | null>(null)
    const [status, setStatus] = useState<string>("")
    const [page, setPage] = useState(0)
    const syncType = selectedType && syncTypes.includes(selectedType) ? selectedType : syncTypes[0]

    const queue = useConnectionQueue(connectionId, {
        syncType,
        status: status || undefined,
        offset: page * PAGE_SIZE,
        pageSize: PAGE_SIZE,
        enabled: open && Boolean(syncType),
    })
    const entries = queue.data?.entries ?? []
    const total = queue.data?.total ?? 0
    const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1)

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-h-[85dvh] w-4xl max-w-[calc(100%-2rem)] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Colas de la conexión</DialogTitle>
                    <DialogDescription>
                        Avance de lo que se está sincronizando en segundo plano. Se actualiza solo mientras haya
                        elementos pendientes.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-wrap items-center gap-2">
                    {syncTypes.map((type) => (
                        <Button
                            key={type}
                            size="sm"
                            variant={type === syncType ? "default" : "outline"}
                            onClick={() => {
                                setSelectedType(type)
                                setPage(0)
                            }}
                        >
                            {syncTypeLabel(type)}
                        </Button>
                    ))}
                    <span className="mx-2 h-5 w-px bg-border" />
                    {STATUS_FILTERS.map((value) => (
                        <Button
                            key={value || "all"}
                            size="sm"
                            variant={value === status ? "secondary" : "ghost"}
                            onClick={() => {
                                setStatus(value)
                                setPage(0)
                            }}
                        >
                            {value ? STATUS_LABELS[value] : "Todos"}
                        </Button>
                    ))}
                </div>

                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>SKU</TableHead>
                            <TableHead>Producto</TableHead>
                            <TableHead>Estado</TableHead>
                            <TableHead>Intentos</TableHead>
                            <TableHead>Actualizado</TableHead>
                            <TableHead>Error</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {entries.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={6} className="text-center text-muted-foreground">
                                    {queue.isLoading ? "Cargando…" : "No hay elementos en la cola con este filtro."}
                                </TableCell>
                            </TableRow>
                        ) : (
                            entries.map((entry) => (
                                <TableRow key={entry.id}>
                                    <TableCell className="font-mono text-xs">{entry.sku}</TableCell>
                                    <TableCell className="max-w-56 truncate">{entry.productName}</TableCell>
                                    <TableCell className={entry.status === "failed" ? "text-destructive" : undefined}>
                                        {STATUS_LABELS[entry.status] ?? entry.status}
                                    </TableCell>
                                    <TableCell>{entry.attempts}</TableCell>
                                    <TableCell className="whitespace-nowrap text-xs">{formatDate(entry.updatedAt)}</TableCell>
                                    <TableCell className="max-w-72 whitespace-normal break-words text-xs text-destructive">
                                        {entry.lastError ?? ""}
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>

                <div className="flex items-center justify-between text-sm text-muted-foreground">
                    <span>{total} elemento(s)</span>
                    <div className="flex items-center gap-2">
                        <Button size="sm" variant="outline" disabled={page <= 0} onClick={() => setPage(page - 1)}>
                            Anterior
                        </Button>
                        <span>
                            Página {page + 1} de {lastPage + 1}
                        </span>
                        <Button size="sm" variant="outline" disabled={page >= lastPage} onClick={() => setPage(page + 1)}>
                            Siguiente
                        </Button>
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )
}

/**
 * Panel de la vista de una conexión: muestra el avance de las colas en segundo
 * plano (resincronización, compatibilidades, publicación) como barras de
 * progreso y abre un detalle con cada producto, su estado y el error si falló.
 * No muestra nada mientras la conexión no tenga filas en la cola.
 */
export function ConnectionQueuePanel({ connectionId }: { connectionId: number }) {
    const queue = useConnectionQueue(connectionId)
    const [open, setOpen] = useState(false)
    const summary = queue.data?.summary ?? []

    if (summary.length === 0) {
        return null
    }

    return (
        <div className="flex w-full max-w-xl flex-col gap-3 rounded-lg border p-3">
            <div className="flex items-center justify-between">
                <span className="text-sm font-semibold">Colas de sincronización</span>
                <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
                    <ListTodoIcon />
                    Ver detalle
                </Button>
            </div>
            {summary.map((item) => (
                <ProgressRow key={item.syncType} summary={item} />
            ))}
            <QueueDetailDialog
                connectionId={connectionId}
                syncTypes={summary.map((item) => item.syncType)}
                open={open}
                onOpenChange={setOpen}
            />
        </div>
    )
}

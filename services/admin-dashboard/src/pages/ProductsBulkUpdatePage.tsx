import { PageHeader } from "@/components/PageHeader.tsx"
import { TablePagination } from "@/components/TablePagination"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { useChannelConnections, useStorageDisks } from "@/lib/product-detail"
import {
    useDownloadProductBulkUpdateTemplate,
    useExportProductBulkUpdateBatch,
    useProductBulkUpdateBatches,
    useProductBulkUpdateItems,
    useProductBulkUpdateSummary,
    useProductSources,
    useRetryProductBulkUpdateAll,
    useRetryProductBulkUpdateItem,
    useUploadProductBulkUpdate,
    type ProductBulkUpdateItem,
    type ProductBulkUpdateStatus,
} from "@/lib/product-bulk-update"
import { cn } from "@/lib/utils"
import { DownloadIcon, FileSpreadsheetIcon, Loader2Icon, RotateCcwIcon, SearchIcon, UploadIcon, XIcon } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Link } from "react-router-dom"

type Tab = "queue" | "history"

// Chips de filtro de la pestaña "Actualizaciones". "all" no filtra por estado.
type Filter = "all" | "pending" | "done" | "failed" | "missing"

const FILTERS: { key: Filter; label: string; statuses: ProductBulkUpdateStatus[] }[] = [
    { key: "all", label: "Todos", statuses: [] },
    { key: "pending", label: "Pendientes", statuses: ["pending", "running"] },
    { key: "done", label: "Concluidos", statuses: ["done"] },
    { key: "failed", label: "Con error", statuses: ["failed"] },
    { key: "missing", label: "Faltantes", statuses: ["missing"] },
]

const STATUS_LABEL: Record<ProductBulkUpdateStatus, { label: string; className: string }> = {
    pending: { label: "Pendiente", className: "bg-muted text-muted-foreground" },
    running: { label: "Actualizando…", className: "bg-blue-100 text-blue-800" },
    done: { label: "Concluido", className: "bg-green-100 text-green-800" },
    failed: { label: "Error", className: "bg-red-100 text-red-800" },
    missing: { label: "Faltante", className: "bg-amber-100 text-amber-800" },
}

const STEP_LABEL: Record<string, string> = {
    general: "Datos generales",
    seo: "SEO",
    dimensions: "Medidas",
    images: "Imágenes",
    video: "Video",
    resync: "Sincronización",
}

// Canales que saben resincronizar una publicación ya existente (el servidor
// valida lo mismo; esto solo evita ofrecer conexiones que va a rechazar).
const RESYNC_CHANNEL_CODES = ["MERCADOLIBRE", "ODOO"]

function formatDate(value: string | null | undefined) {
    if (!value) return "—"
    return new Date(value).toLocaleString("es-MX", { dateStyle: "short", timeStyle: "short" })
}

function StatusTag({ status }: { status: ProductBulkUpdateStatus }) {
    const { label, className } = STATUS_LABEL[status]
    return <span className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium", className)}>{label}</span>
}

// Qué se actualizó en la fila: un chip por paso que sí se ejecutó (los pasos
// sin datos en el Excel no se muestran).
function StepsCell({ item }: { item: ProductBulkUpdateItem }) {
    const steps = (item.steps ?? []).filter((step) => step.status !== "skipped")
    if (steps.length === 0) return <span className="text-muted-foreground">—</span>
    return (
        <div className="flex max-w-64 flex-wrap gap-1 whitespace-normal">
            {steps.map((step) => (
                <span
                    key={step.step}
                    title={step.detail ?? undefined}
                    className={cn(
                        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
                        step.status === "error" ? "bg-red-100 text-red-800" : "bg-green-100 text-green-800",
                    )}
                >
                    {STEP_LABEL[step.step] ?? step.step}
                </span>
            ))}
        </div>
    )
}

export function ProductsBulkUpdatePage() {
    const [tab, setTab] = useState<Tab>("queue")
    const [filter, setFilter] = useState<Filter>("pending")
    const [sourceId, setSourceId] = useState<number | null>(null)
    const [storageDiskId, setStorageDiskId] = useState<number | null>(null)
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(25)
    const [searchInput, setSearchInput] = useState("")
    const [search, setSearch] = useState("")
    const [uploadMessage, setUploadMessage] = useState<{ ok: boolean; text: string } | null>(null)
    const fileInput = useRef<HTMLInputElement>(null)

    const sourcesQuery = useProductSources()
    const sources = sourcesQuery.data ?? []
    const disksQuery = useStorageDisks()
    const disks = disksQuery.data ?? []
    // Con una sola source / un solo disco no hay nada que elegir.
    const selectedSourceId = sourceId ?? (sources.length === 1 ? sources[0].id : null)
    const selectedDiskId = storageDiskId ?? (disks.length === 1 ? disks[0].id : null)

    // Conexiones donde, además de la base local, se resincroniza cada producto.
    const connectionsQuery = useChannelConnections()
    const syncableConnections = useMemo(
        () =>
            (connectionsQuery.data ?? []).filter(
                (c) => c.status === "active" && !c.deletedAt && RESYNC_CHANNEL_CODES.includes(c.channelCode),
            ),
        [connectionsQuery.data],
    )
    const [syncConnectionIds, setSyncConnectionIds] = useState<number[]>([])
    function toggleSyncConnection(id: number, checked: boolean) {
        setSyncConnectionIds((current) => (checked ? [...current, id] : current.filter((existing) => existing !== id)))
    }

    useEffect(() => {
        const timer = setTimeout(() => {
            if (searchInput.trim() !== search) {
                setSearch(searchInput.trim())
                setOffset(0)
            }
        }, 300)
        return () => clearTimeout(timer)
    }, [searchInput, search])

    const summaryQuery = useProductBulkUpdateSummary()
    const counts = summaryQuery.data ?? {}
    const inProgress = (counts.pending ?? 0) + (counts.running ?? 0)

    const activeFilter = FILTERS.find((f) => f.key === filter) ?? FILTERS[0]
    const itemsQuery = useProductBulkUpdateItems(
        {
            statuses: tab === "history" ? ["done"] : activeFilter.statuses,
            search,
            history: tab === "history",
            offset,
            pageSize,
        },
        inProgress > 0,
    )
    const items = itemsQuery.data?.items ?? []
    const total = itemsQuery.data?.total ?? 0

    const upload = useUploadProductBulkUpdate()
    const retryItem = useRetryProductBulkUpdateItem()
    const retryAll = useRetryProductBulkUpdateAll()
    const batchesQuery = useProductBulkUpdateBatches()
    const exportBatch = useExportProductBulkUpdateBatch()
    const template = useDownloadProductBulkUpdateTemplate()
    const [exportBatchId, setExportBatchId] = useState<number | null>(null)

    function countFor(key: Filter): number {
        const statuses = FILTERS.find((f) => f.key === key)?.statuses ?? []
        if (statuses.length === 0) return Object.values(counts).reduce((a, b) => a + b, 0)
        return statuses.reduce((sum, status) => sum + (counts[status] ?? 0), 0)
    }

    function handleFile(file: File | undefined) {
        if (!file || selectedSourceId == null || selectedDiskId == null) return
        setUploadMessage(null)
        upload.mutate(
            { sourceId: selectedSourceId, storageDiskId: selectedDiskId, syncConnectionIds, file },
            {
                onSuccess: (result) => {
                    setUploadMessage({
                        ok: true,
                        text:
                            `${result.totalRows} filas leídas: ${result.queued} en cola` +
                            (result.missing ? `, ${result.missing} faltantes` : "") +
                            (result.invalid ? `, ${result.invalid} inválidas` : "") +
                            "." +
                            (syncConnectionIds.length
                                ? ` Se resincronizarán en ${syncConnectionIds.length} conexión(es) al actualizar cada producto.`
                                : ""),
                    })
                    setTab("queue")
                    setFilter("all")
                    setOffset(0)
                },
                onError: (error) => setUploadMessage({ ok: false, text: getServerErrorMessage(error, "No se pudo subir el archivo.") }),
            },
        )
    }

    const showFilters = tab === "queue"
    const canUpload = selectedSourceId != null && selectedDiskId != null && !upload.isPending

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Actualización masiva de productos</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-4">
                <div className="flex shrink-0 flex-wrap items-end gap-4">
                    <div className="flex flex-col gap-1">
                        <label htmlFor="bulk-update-source" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Source de los productos
                        </label>
                        <select
                            id="bulk-update-source"
                            value={selectedSourceId ?? ""}
                            onChange={(event) => setSourceId(event.target.value ? Number(event.target.value) : null)}
                            className="h-9 min-w-56 rounded-md border border-input bg-transparent px-2 text-sm"
                        >
                            <option value="">Selecciona una source…</option>
                            {sources.map((source) => (
                                <option key={source.id} value={source.id}>
                                    {source.name}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="flex flex-col gap-1">
                        <label htmlFor="bulk-update-disk" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Disco de almacenamiento
                        </label>
                        <select
                            id="bulk-update-disk"
                            value={selectedDiskId ?? ""}
                            onChange={(event) => setStorageDiskId(event.target.value ? Number(event.target.value) : null)}
                            className="h-9 min-w-56 rounded-md border border-input bg-transparent px-2 text-sm"
                        >
                            <option value="">Selecciona un disco…</option>
                            {disks.map((disk) => (
                                <option key={disk.id} value={disk.id}>
                                    {disk.name}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Excel con los datos a actualizar</span>
                        <div className="flex items-center gap-2">
                            <input
                                ref={fileInput}
                                type="file"
                                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                                className="hidden"
                                onChange={(event) => {
                                    handleFile(event.target.files?.[0])
                                    event.target.value = ""
                                }}
                            />
                            <Button variant="outline" size="sm" disabled={!canUpload} onClick={() => fileInput.current?.click()}>
                                {upload.isPending ? <Loader2Icon className="animate-spin" /> : <UploadIcon />}
                                {upload.isPending ? "Subiendo…" : "Seleccionar archivo Excel"}
                            </Button>
                            <Button variant="ghost" size="sm" disabled={template.isPending} onClick={() => template.mutate()}>
                                {template.isPending ? <Loader2Icon className="animate-spin" /> : <FileSpreadsheetIcon />}
                                Descargar plantilla
                            </Button>
                        </div>
                    </div>
                    <div className="flex flex-col gap-1">
                        <label htmlFor="bulk-update-export-batch" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Reporte de un lote
                        </label>
                        <div className="flex items-center gap-2">
                            <select
                                id="bulk-update-export-batch"
                                value={exportBatchId ?? ""}
                                onChange={(event) => setExportBatchId(event.target.value ? Number(event.target.value) : null)}
                                className="h-9 min-w-56 max-w-80 rounded-md border border-input bg-transparent px-2 text-sm"
                            >
                                <option value="">Selecciona un archivo…</option>
                                {(batchesQuery.data ?? []).map((batch) => (
                                    <option key={batch.id} value={batch.id}>
                                        {batch.fileName} · {formatDate(batch.createdAt)} ({batch.totalRows})
                                    </option>
                                ))}
                            </select>
                            <Button
                                variant="outline"
                                size="sm"
                                disabled={exportBatchId == null || exportBatch.isPending}
                                onClick={() => exportBatchId != null && exportBatch.mutate(exportBatchId)}
                            >
                                {exportBatch.isPending ? <Loader2Icon className="animate-spin" /> : <DownloadIcon />}
                                {exportBatch.isPending ? "Generando…" : "Exportar Excel"}
                            </Button>
                        </div>
                    </div>
                    {inProgress > 0 ? (
                        <p className="flex items-center gap-1.5 pb-2 text-sm text-muted-foreground">
                            <Loader2Icon className="size-4 animate-spin" />
                            Quedan {inProgress} por actualizar; se procesan en el servidor.
                        </p>
                    ) : null}
                </div>
                <fieldset className="flex shrink-0 flex-col gap-1.5" disabled={upload.isPending}>
                    <legend className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                        Resincronizar en las conexiones
                    </legend>
                    {syncableConnections.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                            {connectionsQuery.isLoading ? "Cargando conexiones…" : "No hay conexiones activas que permitan resincronizar."}
                        </p>
                    ) : (
                        <div className="flex flex-wrap gap-x-5 gap-y-1.5">
                            {syncableConnections.map((connection) => (
                                <label key={connection.id} className="flex cursor-pointer items-center gap-2 text-sm">
                                    <input
                                        type="checkbox"
                                        className="size-4"
                                        checked={syncConnectionIds.includes(connection.id)}
                                        onChange={(event) => toggleSyncConnection(connection.id, event.target.checked)}
                                    />
                                    {connection.name}
                                    {connection.channelName ? (
                                        <span className="text-xs text-muted-foreground">({connection.channelName})</span>
                                    ) : null}
                                </label>
                            ))}
                        </div>
                    )}
                    <p className="text-xs text-muted-foreground">
                        Opcional. Después de guardar cada producto en la base local se actualizan sus publicaciones en las
                        conexiones marcadas (solo las que ya existen; no se crean publicaciones nuevas).
                    </p>
                </fieldset>
                {exportBatch.isError ? (
                    <p className="shrink-0 text-sm text-destructive">{getServerErrorMessage(exportBatch.error, "No se pudo generar el reporte.")}</p>
                ) : null}
                {template.isError ? (
                    <p className="shrink-0 text-sm text-destructive">{getServerErrorMessage(template.error, "No se pudo descargar la plantilla.")}</p>
                ) : null}
                {uploadMessage ? (
                    <p className={cn("shrink-0 text-sm", uploadMessage.ok ? "text-green-700" : "text-destructive")}>{uploadMessage.text}</p>
                ) : null}

                <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-line pb-2">
                    {(
                        [
                            ["queue", "Actualizaciones"],
                            ["history", `Historial (${counts.done ?? 0})`],
                        ] as [Tab, string][]
                    ).map(([key, label]) => (
                        <Button
                            key={key}
                            variant={tab === key ? "default" : "ghost"}
                            size="sm"
                            onClick={() => {
                                setTab(key)
                                setOffset(0)
                            }}
                        >
                            {label}
                        </Button>
                    ))}
                    <div className="relative ml-auto w-full max-w-xs">
                        <SearchIcon className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            value={searchInput}
                            onChange={(event) => setSearchInput(event.target.value)}
                            placeholder="Buscar por SKU o nombre"
                            aria-label="Buscar"
                            className="px-8"
                        />
                        {searchInput ? (
                            <button
                                type="button"
                                onClick={() => setSearchInput("")}
                                aria-label="Borrar búsqueda"
                                className="absolute right-1.5 top-1/2 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground"
                            >
                                <XIcon className="size-4" />
                            </button>
                        ) : null}
                    </div>
                </div>

                {showFilters ? (
                    <div className="flex shrink-0 flex-wrap items-center gap-2">
                        {FILTERS.map((f) => (
                            <Button
                                key={f.key}
                                variant={filter === f.key ? "secondary" : "outline"}
                                size="sm"
                                onClick={() => {
                                    setFilter(f.key)
                                    setOffset(0)
                                }}
                            >
                                {f.label} ({countFor(f.key)})
                            </Button>
                        ))}
                        {filter === "failed" || filter === "missing" ? (
                            <Button
                                variant="outline"
                                size="sm"
                                className="ml-auto"
                                disabled={retryAll.isPending || total === 0}
                                onClick={() => retryAll.mutate({ status: filter as "failed" | "missing" })}
                            >
                                <RotateCcwIcon />
                                {filter === "missing" ? "Reprocesar faltantes" : "Reintentar con error"}
                            </Button>
                        ) : null}
                    </div>
                ) : null}
                {retryAll.isSuccess ? (
                    <p className="shrink-0 text-sm text-muted-foreground">{retryAll.data} fila(s) volvieron a la cola.</p>
                ) : null}
                {retryItem.isError ? (
                    <p className="shrink-0 text-sm text-destructive">{getServerErrorMessage(retryItem.error, "No se pudo reintentar la fila.")}</p>
                ) : null}
                {itemsQuery.isError ? <p className="shrink-0 text-sm text-destructive">No se pudo cargar la lista.</p> : null}

                <div className="min-h-0 flex-1">
                    <Table containerClassName="h-full overflow-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Fila</TableHead>
                                <TableHead>SKU</TableHead>
                                <TableHead>Producto</TableHead>
                                <TableHead>Estado</TableHead>
                                <TableHead>Actualizado</TableHead>
                                <TableHead>{tab === "history" ? "Concluido" : "Archivo"}</TableHead>
                                <TableHead>Detalle</TableHead>
                                <TableHead className="w-24" />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {itemsQuery.isLoading ? (
                                <TableRow>
                                    <TableCell colSpan={8} className="text-center text-muted-foreground">
                                        Cargando…
                                    </TableCell>
                                </TableRow>
                            ) : null}
                            {!itemsQuery.isLoading && items.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={8} className="text-center text-muted-foreground">
                                        {search
                                            ? "No hay filas que coincidan con la búsqueda."
                                            : tab === "history"
                                              ? "Aún no hay actualizaciones concluidas."
                                              : "No hay filas con este filtro. Sube un Excel para empezar."}
                                    </TableCell>
                                </TableRow>
                            ) : null}
                            {items.map((item) => (
                                <TableRow key={item.id}>
                                    <TableCell className="tabular-nums text-muted-foreground">{item.sheetRow}</TableCell>
                                    <TableCell>
                                        {item.productId ? (
                                            <Link to={`/products/${item.productId}`} className="text-primary hover:underline">
                                                {item.sku}
                                            </Link>
                                        ) : (
                                            item.sku || "—"
                                        )}
                                    </TableCell>
                                    <TableCell>{item.productName ?? "—"}</TableCell>
                                    <TableCell>
                                        <StatusTag status={item.status} />
                                    </TableCell>
                                    <TableCell>
                                        <StepsCell item={item} />
                                    </TableCell>
                                    <TableCell className="text-xs text-muted-foreground">
                                        {tab === "history" ? formatDate(item.finishedAt) : item.batchFile}
                                    </TableCell>
                                    <TableCell className="max-w-md whitespace-normal text-xs text-muted-foreground">
                                        {item.errorMessage ?? ""}
                                    </TableCell>
                                    <TableCell>
                                        {item.retryable ? (
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                disabled={retryItem.isPending}
                                                onClick={() => retryItem.mutate(item.id)}
                                            >
                                                <RotateCcwIcon />
                                                Reintentar
                                            </Button>
                                        ) : null}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </div>

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
            </section>
        </main>
    )
}

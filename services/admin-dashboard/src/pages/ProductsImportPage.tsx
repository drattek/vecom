import { PageHeader } from "@/components/PageHeader.tsx"
import { TablePagination } from "@/components/TablePagination"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import {
    useBulkImportItems,
    useBulkImportSummary,
    useRetryBulkImportAll,
    useRetryBulkImportItem,
    useUploadBulkImport,
    type BulkImportItem,
    type BulkImportStatus,
} from "@/lib/meli-bulk-import"
import { useChannelConnections } from "@/lib/product-detail"
import { cn } from "@/lib/utils"
import { Loader2Icon, RotateCcwIcon, SearchIcon, UploadIcon, XIcon } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Link } from "react-router-dom"

type Tab = "queue" | "history"

// Chips de filtro de la pestaña "Importaciones". "all" no filtra por estado.
type Filter = "all" | "pending" | "done" | "failed" | "missing" | "duplicate"

const FILTERS: { key: Filter; label: string; statuses: BulkImportStatus[] }[] = [
    { key: "all", label: "Todos", statuses: [] },
    { key: "pending", label: "Pendientes", statuses: ["pending", "running"] },
    { key: "done", label: "Concluidos", statuses: ["done"] },
    { key: "failed", label: "Con error", statuses: ["failed"] },
    { key: "missing", label: "Faltantes", statuses: ["missing"] },
    { key: "duplicate", label: "Duplicados", statuses: ["duplicate"] },
]

const STATUS_LABEL: Record<BulkImportStatus, { label: string; className: string }> = {
    pending: { label: "Pendiente", className: "bg-muted text-muted-foreground" },
    running: { label: "Importando…", className: "bg-blue-100 text-blue-800" },
    done: { label: "Concluido", className: "bg-green-100 text-green-800" },
    failed: { label: "Error", className: "bg-red-100 text-red-800" },
    missing: { label: "Faltante", className: "bg-amber-100 text-amber-800" },
    duplicate: { label: "Duplicado", className: "bg-slate-200 text-slate-700" },
}

const STEP_LABEL: Record<string, string> = {
    name: "Nombre",
    category: "Categoría",
    description: "Descripción",
    attributes: "Atributos",
    compatibilities: "Compatibilidades",
    images: "Imágenes",
    link: "Vinculación",
    price_stock: "Precio y stock",
}

function formatDate(value: string | null | undefined) {
    if (!value) return "—"
    return new Date(value).toLocaleString("es-MX", { dateStyle: "short", timeStyle: "short" })
}

function StatusTag({ status }: { status: BulkImportStatus }) {
    const { label, className } = STATUS_LABEL[status]
    return <span className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium", className)}>{label}</span>
}

// Resultado del paso "Precio y stock" (lo que se envió a MercadoLibre al vincular):
// ok = enviados, skipped = no se envió (p. ej. sin stock o sin precio), error = falló.
function PriceStockCell({ item }: { item: BulkImportItem }) {
    const step = item.steps?.find((s) => s.step === "price_stock")
    if (!step) return <span className="text-muted-foreground">—</span>

    const tag =
        step.status === "ok"
            ? { label: "Enviados", className: "bg-green-100 text-green-800" }
            : step.status === "error"
              ? { label: "Error", className: "bg-red-100 text-red-800" }
              : { label: "No enviados", className: "bg-amber-100 text-amber-800" }
    return (
        <div className="flex max-w-56 flex-col gap-0.5 whitespace-normal">
            <span className={cn("inline-flex w-fit items-center rounded-full px-2 py-0.5 text-xs font-medium", tag.className)}>
                {tag.label}
            </span>
            {step.detail ? <span className="text-xs text-muted-foreground">{step.detail}</span> : null}
        </div>
    )
}

function stepsSummary(item: BulkImportItem) {
    const problems = (item.steps ?? []).filter(
        (step) => step.step !== "price_stock" && (step.status === "error" || step.status === "warning"),
    )
    if (problems.length === 0) return null
    return problems.map((step) => `${STEP_LABEL[step.step] ?? step.step}: ${step.detail ?? step.status}`).join(" · ")
}

export function ProductsImportPage() {
    const [tab, setTab] = useState<Tab>("queue")
    const [filter, setFilter] = useState<Filter>("pending")
    const [connectionId, setConnectionId] = useState<number | null>(null)
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(25)
    const [searchInput, setSearchInput] = useState("")
    const [search, setSearch] = useState("")
    const [uploadMessage, setUploadMessage] = useState<{ ok: boolean; text: string } | null>(null)
    const fileInput = useRef<HTMLInputElement>(null)

    const connectionsQuery = useChannelConnections()
    const meliConnections = useMemo(
        () => (connectionsQuery.data ?? []).filter((c) => c.channelCode === "MERCADOLIBRE" && !c.deletedAt),
        [connectionsQuery.data],
    )
    // Con una sola conexión de MercadoLibre no hay nada que elegir.
    const selectedConnectionId = connectionId ?? (meliConnections.length === 1 ? meliConnections[0].id : null)

    useEffect(() => {
        const timer = setTimeout(() => {
            if (searchInput.trim() !== search) {
                setSearch(searchInput.trim())
                setOffset(0)
            }
        }, 300)
        return () => clearTimeout(timer)
    }, [searchInput, search])

    const summaryQuery = useBulkImportSummary(null)
    const counts = summaryQuery.data ?? {}
    const inProgress = (counts.pending ?? 0) + (counts.running ?? 0)

    const activeFilter = FILTERS.find((f) => f.key === filter) ?? FILTERS[0]
    const itemsQuery = useBulkImportItems(
        {
            statuses: tab === "history" ? ["done"] : activeFilter.statuses,
            connectionId: null,
            search,
            history: tab === "history",
            offset,
            pageSize,
        },
        inProgress > 0,
    )
    const items = itemsQuery.data?.items ?? []
    const total = itemsQuery.data?.total ?? 0

    const upload = useUploadBulkImport()
    const retryItem = useRetryBulkImportItem()
    const retryAll = useRetryBulkImportAll()

    function countFor(key: Filter): number {
        const statuses = FILTERS.find((f) => f.key === key)?.statuses ?? []
        if (statuses.length === 0) return Object.values(counts).reduce((a, b) => a + b, 0)
        return statuses.reduce((sum, status) => sum + (counts[status] ?? 0), 0)
    }

    function handleFile(file: File | undefined) {
        if (!file || selectedConnectionId == null) return
        setUploadMessage(null)
        upload.mutate(
            { connectionId: selectedConnectionId, file },
            {
                onSuccess: (result) => {
                    setUploadMessage({
                        ok: true,
                        text:
                            `${result.totalRows} filas leídas: ${result.queued} en cola` +
                            (result.missing ? `, ${result.missing} faltantes` : "") +
                            (result.invalid ? `, ${result.invalid} inválidas` : "") +
                            ".",
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

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Importar publicaciones de MercadoLibre</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-4">
                <div className="flex shrink-0 flex-wrap items-end gap-4">
                    <div className="flex flex-col gap-1">
                        <label htmlFor="bulk-import-connection" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Conexión de MercadoLibre
                        </label>
                        <select
                            id="bulk-import-connection"
                            value={selectedConnectionId ?? ""}
                            onChange={(event) => setConnectionId(event.target.value ? Number(event.target.value) : null)}
                            className="h-9 min-w-56 rounded-md border border-input bg-transparent px-2 text-sm"
                        >
                            <option value="">Selecciona una conexión…</option>
                            {meliConnections.map((connection) => (
                                <option key={connection.id} value={connection.id}>
                                    {connection.name}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Excel con columnas sku y meli_id
                        </span>
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
                        <Button
                            variant="outline"
                            size="sm"
                            disabled={selectedConnectionId == null || upload.isPending}
                            onClick={() => fileInput.current?.click()}
                        >
                            {upload.isPending ? <Loader2Icon className="animate-spin" /> : <UploadIcon />}
                            {upload.isPending ? "Subiendo…" : "Seleccionar archivo Excel"}
                        </Button>
                    </div>
                    {inProgress > 0 ? (
                        <p className="flex items-center gap-1.5 pb-2 text-sm text-muted-foreground">
                            <Loader2Icon className="size-4 animate-spin" />
                            Quedan {inProgress} por importar; se procesan en el servidor de una en una.
                        </p>
                    ) : null}
                </div>
                {uploadMessage ? (
                    <p className={cn("shrink-0 text-sm", uploadMessage.ok ? "text-green-700" : "text-destructive")}>{uploadMessage.text}</p>
                ) : null}

                <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-line pb-2">
                    {(
                        [
                            ["queue", "Importaciones"],
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
                            placeholder="Buscar por SKU, meli_id o nombre"
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
                                onClick={() => retryAll.mutate({ status: filter as "failed" | "missing", connectionId: null })}
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
                                <TableHead>SKU</TableHead>
                                <TableHead>meli_id</TableHead>
                                <TableHead>Producto</TableHead>
                                <TableHead>Estado</TableHead>
                                <TableHead>Precio y stock</TableHead>
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
                                              ? "Aún no hay importaciones concluidas."
                                              : "No hay filas con este filtro. Sube un Excel para empezar."}
                                    </TableCell>
                                </TableRow>
                            ) : null}
                            {items.map((item) => (
                                <TableRow key={item.id}>
                                    <TableCell>
                                        {item.productId ? (
                                            <Link to={`/products/${item.productId}`} className="text-primary hover:underline">
                                                {item.sku}
                                            </Link>
                                        ) : (
                                            item.sku || "—"
                                        )}
                                    </TableCell>
                                    <TableCell>{item.externalId ?? item.meliId ?? "—"}</TableCell>
                                    <TableCell>{item.productName ?? item.listingTitle ?? "—"}</TableCell>
                                    <TableCell>
                                        <StatusTag status={item.status} />
                                    </TableCell>
                                    <TableCell>
                                        <PriceStockCell item={item} />
                                    </TableCell>
                                    <TableCell className="text-xs text-muted-foreground">
                                        {tab === "history" ? formatDate(item.finishedAt) : item.batchFile}
                                    </TableCell>
                                    <TableCell className="max-w-md whitespace-normal text-xs text-muted-foreground">
                                        {item.errorMessage ?? stepsSummary(item) ?? ""}
                                    </TableCell>
                                    <TableCell>
                                        {item.status === "failed" || item.status === "missing" ? (
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

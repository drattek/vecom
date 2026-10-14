import { useState } from "react"
import { Link } from "react-router-dom"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { ExternalLinkIcon, HistoryIcon, SearchIcon } from "lucide-react"

import { TablePagination } from "@/components/TablePagination"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { apiClient } from "@/lib/api"
import { paginatedChannelConnectionsResponseSchema } from "@/lib/schemas/channel-connections"
import {
    listingHistoryResponseSchema,
    paginatedListingViewSchema,
    type ListingEvent,
    type ListingState,
    type ListingViewRow,
} from "@/lib/schemas/channel-listings-view"

const MELI_CHANNEL_CODE = "MERCADOLIBRE"
const ALL = "all"

// Orden y textos de los estados que mostramos. "not_published" son productos
// cuyo último intento de publicar falló y nunca tuvieron publicación.
const STATES: { key: ListingState; label: string; tone: string }[] = [
    { key: "published", label: "Publicadas", tone: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400" },
    { key: "paused_zero_stock", label: "Pausadas sin stock", tone: "bg-amber-500/10 text-amber-700 dark:text-amber-400" },
    { key: "paused_manual", label: "Pausadas manual", tone: "bg-orange-500/10 text-orange-700 dark:text-orange-400" },
    { key: "under_review", label: "En revisión MELI", tone: "bg-sky-500/10 text-sky-700 dark:text-sky-400" },
    { key: "not_published", label: "No publicadas", tone: "bg-destructive/10 text-destructive" },
    { key: "error", label: "Con error", tone: "bg-destructive/10 text-destructive" },
    { key: "pending", label: "Pendientes", tone: "bg-muted text-muted-foreground" },
    { key: "closed", label: "Cerradas", tone: "bg-muted text-muted-foreground" },
]

const STATE_BY_KEY = Object.fromEntries(STATES.map((s) => [s.key, s])) as Record<ListingState, (typeof STATES)[number]>

const ACTION_LABEL: Record<ListingEvent["action"], string> = {
    publish: "Publicación",
    refresh: "Refresco precio/stock",
    resync: "Resincronización",
    pause: "Pausa automática",
    reactivate: "Reactivación",
    status_sync: "Cambio de estado (MELI)",
}

function formatDate(value?: string | null): string {
    if (!value) return "-"
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) return "-"
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

function meliPermalink(externalId?: string | null): string | null {
    if (!externalId) return null
    const m = /^([A-Z]{3})-?(\d+)$/.exec(externalId.trim())
    if (!m) return null
    return `https://articulo.mercadolibre.com.mx/${m[1]}-${m[2]}`
}

function StateBadge({ state }: { state: ListingState }) {
    const info = STATE_BY_KEY[state]
    return (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ${info?.tone ?? ""}`}>
            {info?.label ?? state}
        </span>
    )
}

export function MeliListingsPage() {
    const [connectionId, setConnectionId] = useState<string>(ALL)
    const [state, setState] = useState<ListingState | "">("")
    const [withError, setWithError] = useState(false)
    const [search, setSearch] = useState("")
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(50)
    const [historyRow, setHistoryRow] = useState<ListingViewRow | null>(null)

    const connections = useQuery({
        queryKey: ["channel-connections", "meli-filter"],
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-connections", { params: { offset: 0, pageSize: 100 } })
            return paginatedChannelConnectionsResponseSchema
                .parse(response.data)
                .channelConnections.filter((c) => c.channelCode.toUpperCase() === MELI_CHANNEL_CODE)
        },
        staleTime: 60_000,
    })

    const listings = useQuery({
        queryKey: ["meli-listings", connectionId, state, withError, search, offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-listings", {
                params: {
                    channelCode: MELI_CHANNEL_CODE,
                    connectionId: connectionId === ALL ? undefined : connectionId,
                    state: state || undefined,
                    withError: withError || undefined,
                    search: search.trim() || undefined,
                    offset,
                    pageSize,
                },
            })
            return paginatedListingViewSchema.parse(response.data)
        },
    })

    const connectionItems = [
        { value: ALL, label: "Todas las conexiones" },
        ...(connections.data ?? []).map((c) => ({ value: String(c.id), label: c.name })),
    ]

    const counts = listings.data?.stateCounts ?? {}
    const totalAll = Object.values(counts).reduce((a, b) => a + b, 0)
    const rows = listings.data?.listings ?? []

    function pickState(next: ListingState | "") {
        setState((prev) => (prev === next ? "" : next))
        setOffset(0)
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">MercadoLibre · Publicaciones</h2>
            <p className="mt-1 text-sm text-muted-foreground">
                Todas las publicaciones creadas por el middleware, su estado en MercadoLibre y los errores. Da clic en una
                fila para ver su historial completo de intentos.
            </p>

            <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-9">
                <button
                    type="button"
                    onClick={() => pickState("")}
                    className={`rounded-lg border p-3 text-left transition-colors hover:bg-muted ${state === "" ? "border-primary" : "border-border"}`}
                >
                    <div className="text-xs text-muted-foreground">Total</div>
                    <div className="text-xl font-semibold tabular-nums">{totalAll.toLocaleString("es-MX")}</div>
                </button>
                {STATES.map((s) => (
                    <button
                        key={s.key}
                        type="button"
                        onClick={() => pickState(s.key)}
                        className={`rounded-lg border p-3 text-left transition-colors hover:bg-muted ${state === s.key ? "border-primary" : "border-border"}`}
                    >
                        <div className="text-xs text-muted-foreground">{s.label}</div>
                        <div className="text-xl font-semibold tabular-nums">{(counts[s.key] ?? 0).toLocaleString("es-MX")}</div>
                    </button>
                ))}
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-2">
                <div className="relative min-w-60 flex-1">
                    <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={search}
                        onChange={(e) => {
                            setSearch(e.target.value)
                            setOffset(0)
                        }}
                        placeholder="Buscar por SKU, nombre, título o MLM"
                        aria-label="Buscar publicaciones"
                        className="pl-8"
                    />
                </div>
                <Select
                    value={connectionId}
                    onValueChange={(value) => {
                        setConnectionId(value ?? ALL)
                        setOffset(0)
                    }}
                    items={connectionItems}
                >
                    <SelectTrigger className="w-56" aria-label="Filtrar por conexión">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {connectionItems.map((c) => (
                                <SelectItem key={c.value} value={c.value}>
                                    {c.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <label className="flex items-center gap-2 text-sm">
                    <input
                        type="checkbox"
                        className="size-4"
                        checked={withError}
                        onChange={(e) => {
                            setWithError(e.target.checked)
                            setOffset(0)
                        }}
                    />
                    Solo con error ({(listings.data?.withErrorCount ?? 0).toLocaleString("es-MX")})
                </label>
            </div>

            {listings.isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al cargar publicaciones: {listings.error instanceof Error ? listings.error.message : "desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-3 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>SKU</TableHead>
                        <TableHead>Producto / título</TableHead>
                        <TableHead>Conexión</TableHead>
                        <TableHead>Publicación</TableHead>
                        <TableHead>Estado</TableHead>
                        <TableHead>Error</TableHead>
                        <TableHead className="text-right">Intentos</TableHead>
                        <TableHead>Última actividad</TableHead>
                        <TableHead></TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {listings.isLoading && rows.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={9} className="text-center text-muted-foreground">
                                Cargando publicaciones...
                            </TableCell>
                        </TableRow>
                    ) : null}
                    {!listings.isLoading && rows.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={9} className="text-center text-muted-foreground">
                                No hay publicaciones con estos filtros.
                            </TableCell>
                        </TableRow>
                    ) : null}
                    {rows.map((row) => {
                        const link = meliPermalink(row.externalId)
                        return (
                            <TableRow
                                key={`${row.kind}-${row.id}`}
                                className="cursor-pointer"
                                onClick={() => setHistoryRow(row)}
                            >
                                <TableCell className="font-mono text-xs">
                                    <Link
                                        to={`/products/${row.productId}`}
                                        className="hover:underline"
                                        onClick={(e) => e.stopPropagation()}
                                    >
                                        {row.sku}
                                    </Link>
                                </TableCell>
                                <TableCell className="max-w-72">
                                    <div className="truncate" title={row.listingTitle ?? row.productName}>
                                        {row.listingTitle ?? row.productName}
                                    </div>
                                </TableCell>
                                <TableCell className="whitespace-nowrap">{row.connectionName}</TableCell>
                                <TableCell className="whitespace-nowrap">
                                    {link ? (
                                        <a
                                            href={link}
                                            target="_blank"
                                            rel="noreferrer"
                                            className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
                                            onClick={(e) => e.stopPropagation()}
                                        >
                                            {row.externalId}
                                            <ExternalLinkIcon className="size-3" />
                                        </a>
                                    ) : (
                                        <span className="text-xs text-muted-foreground">sin publicar</span>
                                    )}
                                </TableCell>
                                <TableCell>
                                    <StateBadge state={row.state} />
                                </TableCell>
                                <TableCell className="max-w-80">
                                    {row.lastError ? (
                                        <div className="truncate text-xs text-destructive" title={row.lastError}>
                                            {row.lastError}
                                        </div>
                                    ) : (
                                        <span className="text-xs text-muted-foreground">-</span>
                                    )}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">{row.attempts}</TableCell>
                                <TableCell className="whitespace-nowrap text-xs">{formatDate(row.lastActivityAt)}</TableCell>
                                <TableCell>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            setHistoryRow(row)
                                        }}
                                        title="Ver historial"
                                    >
                                        <HistoryIcon />
                                    </Button>
                                </TableCell>
                            </TableRow>
                        )
                    })}
                </TableBody>
            </Table>

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={listings.data?.total ?? 0}
                onOffsetChange={setOffset}
                onPageSizeChange={(size) => {
                    setPageSize(size)
                    setOffset(0)
                }}
                pageSizeOptions={[25, 50, 100, 200]}
            />

            <ListingHistorySheet row={historyRow} onClose={() => setHistoryRow(null)} />
        </section>
    )
}

function ListingHistorySheet({ row, onClose }: { row: ListingViewRow | null; onClose: () => void }) {
    const history = useQuery({
        queryKey: ["meli-listing-history", row?.productId, row?.connectionId, row?.vehicleFitmentId ?? null],
        enabled: row !== null,
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-listings/history", {
                params: {
                    productId: row!.productId,
                    connectionId: row!.connectionId,
                    vehicleFitmentId: row!.vehicleFitmentId ?? undefined,
                },
            })
            return listingHistoryResponseSchema.parse(response.data).events
        },
    })

    const link = meliPermalink(row?.externalId)

    return (
        <Sheet open={row !== null} onOpenChange={(open) => (!open ? onClose() : undefined)}>
            <SheetContent className="w-full sm:max-w-xl data-[side=right]:sm:max-w-xl">
                <SheetHeader>
                    <SheetTitle>Historial · {row?.sku}</SheetTitle>
                    <SheetDescription>
                        {row?.listingTitle ?? row?.productName}
                        <br />
                        {row?.connectionName}
                        {link ? (
                            <>
                                {" · "}
                                <a href={link} target="_blank" rel="noreferrer" className="text-primary hover:underline">
                                    {row?.externalId}
                                </a>
                            </>
                        ) : null}
                    </SheetDescription>
                </SheetHeader>

                <div className="min-h-0 flex-1 overflow-auto px-4 pb-4">
                    {row ? (
                        <div className="mb-4 flex items-center gap-2 text-sm">
                            Estado actual: <StateBadge state={row.state} />
                        </div>
                    ) : null}

                    {history.isLoading ? <p className="text-sm text-muted-foreground">Cargando historial...</p> : null}
                    {history.isError ? <p className="text-sm text-destructive">No se pudo cargar el historial.</p> : null}
                    {history.data && history.data.length === 0 ? (
                        <p className="text-sm text-muted-foreground">Sin eventos registrados.</p>
                    ) : null}

                    <ol className="relative space-y-4 border-l border-border pl-5">
                        {(history.data ?? []).map((ev) => (
                            <li key={ev.id} className="relative">
                                <span
                                    className={`absolute top-1.5 -left-[25px] size-2.5 rounded-full ${ev.outcome === "error" ? "bg-destructive" : "bg-emerald-500"}`}
                                />
                                <div className="flex flex-wrap items-baseline gap-x-2">
                                    <span className="text-sm font-medium">{ACTION_LABEL[ev.action]}</span>
                                    <span className={`text-xs ${ev.outcome === "error" ? "text-destructive" : "text-emerald-600 dark:text-emerald-400"}`}>
                                        {ev.outcome === "error" ? "falló" : "ok"}
                                    </span>
                                    <span className="text-xs text-muted-foreground">{formatDate(ev.createdAt)}</span>
                                </div>
                                {ev.statusBefore || ev.statusAfter ? (
                                    <div className="text-xs text-muted-foreground">
                                        estado: {ev.statusBefore ?? "—"} → {ev.statusAfter ?? "—"}
                                    </div>
                                ) : null}
                                {ev.externalId ? <div className="font-mono text-xs text-muted-foreground">{ev.externalId}</div> : null}
                                {ev.message ? (
                                    <p
                                        className={`mt-1 text-xs break-words whitespace-pre-wrap ${ev.outcome === "error" ? "text-destructive" : "text-foreground"}`}
                                    >
                                        {ev.message}
                                    </p>
                                ) : null}
                            </li>
                        ))}
                    </ol>
                </div>
            </SheetContent>
        </Sheet>
    )
}

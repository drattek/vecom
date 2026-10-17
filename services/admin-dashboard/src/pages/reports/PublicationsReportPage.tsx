import { useState } from "react"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { CheckIcon, SearchIcon, XIcon } from "lucide-react"

import { TablePagination } from "@/components/TablePagination"
import { formatPrice, ProductSKULink } from "@/components/products/ProductTableHelpers"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { apiClient } from "@/lib/api"
import type { ListingState } from "@/lib/schemas/channel-listings-view"
import { paginatedChannelConnectionsResponseSchema } from "@/lib/schemas/channel-connections"
import {
    paginatedPublicationsReportSchema,
    sourcesResponseSchema,
    type PublicationReportAttributeStatus,
    type PublicationReportConnection,
} from "@/lib/schemas/publications-report"

const ALL = "all"
const CHANNEL_ODOO = "ODOO"
const CHANNEL_MELI = "MERCADOLIBRE"

// Valores del selector "Falta en": "channel:ODOO" (cualquier conexión del
// canal) o "connection:12" (una conexión en específico).
const MISSING_CHANNEL = "channel:"
const MISSING_CONNECTION = "connection:"

const CHANNEL_ITEMS = [
    { value: ALL, label: "Todos los canales" },
    { value: CHANNEL_ODOO, label: "Odoo" },
    { value: CHANNEL_MELI, label: "MercadoLibre" },
]

const STATE_LABEL: Partial<Record<ListingState, { label: string; tone: string }>> = {
    published: { label: "Publicada", tone: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400" },
    paused_zero_stock: { label: "Pausada sin stock", tone: "bg-amber-500/10 text-amber-700 dark:text-amber-400" },
    paused_manual: { label: "Pausada", tone: "bg-orange-500/10 text-orange-700 dark:text-orange-400" },
    under_review: { label: "En revisión", tone: "bg-sky-500/10 text-sky-700 dark:text-sky-400" },
    closed: { label: "Cerrada", tone: "bg-muted text-muted-foreground" },
}

// Una publicación cerrada se muestra en la columna de estado, pero ya no
// cuenta como "sincronizado" en el canal.
function liveConnections(connections: PublicationReportConnection[]): PublicationReportConnection[] {
    return connections.filter((c) => c.state !== "closed")
}

function YesNo({ value }: { value: boolean }) {
    return value ? (
        <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 dark:text-emerald-400">
            <CheckIcon className="size-3.5" /> Sí
        </span>
    ) : (
        <span className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <XIcon className="size-3.5" /> No
        </span>
    )
}

function ConnectionNames({ connections }: { connections: PublicationReportConnection[] }) {
    if (connections.length === 0) return null
    return (
        <ul className="mt-1 space-y-0.5 text-xs text-muted-foreground">
            {connections.map((c) => (
                <li key={c.connectionId} className="whitespace-nowrap">
                    {c.connectionName}
                    {c.listings > 1 ? ` (${c.listings})` : ""}
                </li>
            ))}
        </ul>
    )
}

function SummaryCard({ label, value, hint, children }: { label: string; value?: number; hint?: string; children?: React.ReactNode }) {
    return (
        <div className="rounded-lg border border-border p-3">
            <div className="text-xs text-muted-foreground">{label}</div>
            {value !== undefined ? (
                <div className="text-2xl font-semibold tabular-nums">{value.toLocaleString("es-MX")}</div>
            ) : null}
            {children}
            {hint ? <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div> : null}
        </div>
    )
}

function AttributeStatuses({ statuses }: { statuses: PublicationReportAttributeStatus[] }) {
    if (statuses.length === 0) return <span className="text-xs text-muted-foreground">—</span>
    return (
        <div className="flex flex-col items-start gap-1">
            {statuses.map((a) => {
                let label = "Completos"
                let tone = "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
                let title = `${a.connectionName}: sin atributos requeridos pendientes`
                if (a.state === "needs_category") {
                    label = "Sin categoría"
                    tone = "bg-muted text-muted-foreground"
                    title = `${a.connectionName}: no hay categoría de MercadoLibre resuelta`
                } else if (a.state === "error") {
                    label = "Error"
                    tone = "bg-destructive/10 text-destructive"
                    title = `${a.connectionName}: no se pudo calcular`
                } else if (a.requiredMissing > 0) {
                    label = `Faltan ${a.requiredMissing}`
                    tone = "bg-destructive/10 text-destructive"
                    title = `${a.connectionName}: ${a.missingNames.join(", ")}`
                }
                return (
                    <span
                        key={a.connectionId}
                        title={title}
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ${tone}`}
                    >
                        {label}
                    </span>
                )
            })}
        </div>
    )
}

function ConnectionStates({ connections }: { connections: PublicationReportConnection[] }) {
    if (connections.length === 0) return <span className="text-xs text-muted-foreground">—</span>
    return (
        <div className="flex flex-col items-start gap-1">
            {connections.map((c) => {
                const info = STATE_LABEL[c.state]
                return (
                    <span
                        key={c.connectionId}
                        title={c.connectionName}
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ${info?.tone ?? ""}`}
                    >
                        {info?.label ?? c.state}
                    </span>
                )
            })}
        </div>
    )
}

export function PublicationsReportPage() {
    const [sourceId, setSourceId] = useState<string>(ALL)
    const [channelCode, setChannelCode] = useState<string>(ALL)
    const [connectionId, setConnectionId] = useState<string>(ALL)
    const [search, setSearch] = useState("")
    const [missingIn, setMissingIn] = useState<string>(ALL)
    const [withStockAndPrice, setWithStockAndPrice] = useState(false)
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(50)

    const sources = useQuery({
        queryKey: ["report-sources"],
        queryFn: async () => {
            const response = await apiClient.get("/api/sources", { params: { offset: 0, pageSize: 100 } })
            return sourcesResponseSchema.parse(response.data).sources
        },
        staleTime: 60_000,
    })

    const connections = useQuery({
        queryKey: ["report-connections"],
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-connections", { params: { offset: 0, pageSize: 100 } })
            return paginatedChannelConnectionsResponseSchema.parse(response.data).channelConnections
        },
        staleTime: 60_000,
    })

    const report = useQuery({
        queryKey: ["publications-report", sourceId, channelCode, connectionId, missingIn, withStockAndPrice, search, offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/reports/publications", {
                params: {
                    sourceId: sourceId === ALL ? undefined : sourceId,
                    channelCode: channelCode === ALL ? undefined : channelCode,
                    connectionId: connectionId === ALL ? undefined : connectionId,
                    missingChannelCode: missingIn.startsWith(MISSING_CHANNEL) ? missingIn.slice(MISSING_CHANNEL.length) : undefined,
                    missingConnectionId: missingIn.startsWith(MISSING_CONNECTION) ? missingIn.slice(MISSING_CONNECTION.length) : undefined,
                    withStockAndPrice: withStockAndPrice || undefined,
                    search: search.trim() || undefined,
                    offset,
                    pageSize,
                },
            })
            return paginatedPublicationsReportSchema.parse(response.data)
        },
    })

    const sourceItems = [
        { value: ALL, label: "Todos los sources" },
        ...(sources.data ?? []).map((s) => ({ value: String(s.id), label: s.name })),
    ]

    // Solo Odoo/MercadoLibre tienen columna propia; el filtro de conexión se
    // acota al canal elegido para no ofrecer combinaciones vacías.
    const connectionItems = [
        { value: ALL, label: "Todas las conexiones" },
        ...(connections.data ?? [])
            .filter((c) => {
                const code = c.channelCode.toUpperCase()
                if (channelCode === ALL) return code === CHANNEL_ODOO || code === CHANNEL_MELI
                return code === channelCode
            })
            .map((c) => ({ value: String(c.id), label: `${c.name} (${c.channelName ?? c.channelCode})` })),
    ]

    const reportConnections = (connections.data ?? []).filter((c) => {
        const code = c.channelCode.toUpperCase()
        return code === CHANNEL_ODOO || code === CHANNEL_MELI
    })
    const missingItems = [
        { value: ALL, label: "Sin comparar" },
        { value: `${MISSING_CHANNEL}${CHANNEL_ODOO}`, label: "Falta en: Odoo (cualquier conexión)" },
        { value: `${MISSING_CHANNEL}${CHANNEL_MELI}`, label: "Falta en: MercadoLibre (cualquier conexión)" },
        ...reportConnections.map((c) => ({
            value: `${MISSING_CONNECTION}${c.id}`,
            label: `Falta en: ${c.name} (${c.channelName ?? c.channelCode})`,
        })),
    ]

    const rows = report.data?.rows ?? []
    const summary = report.data?.summary

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Reportes · Publicaciones</h2>
            <p className="mt-1 text-sm text-muted-foreground">
                Productos publicados (activos, en revisión o pausados) en al menos un canal, con el detalle de las
                conexiones donde están sincronizados en Odoo y MercadoLibre.
            </p>

            <div className="mt-4 grid grid-cols-2 gap-2 xl:grid-cols-4">
                <SummaryCard label="Total de publicaciones" value={summary?.total ?? 0} hint="Productos publicados en algún canal" />
                <SummaryCard label="Total en Odoo" value={summary?.inOdoo ?? 0} />
                <SummaryCard label="Total en MercadoLibre" value={summary?.inMercadoLibre ?? 0} />
                <SummaryCard label="Diferencias">
                    <div className="mt-1 space-y-0.5 text-sm">
                        <div>
                            <span className="text-xl font-semibold tabular-nums">
                                {(summary?.mercadoLibreNotOdoo ?? 0).toLocaleString("es-MX")}
                            </span>{" "}
                            <span className="text-xs text-muted-foreground">en MercadoLibre sin Odoo</span>
                        </div>
                        <div>
                            <span className="text-xl font-semibold tabular-nums">
                                {(summary?.odooNotMercadoLibre ?? 0).toLocaleString("es-MX")}
                            </span>{" "}
                            <span className="text-xs text-muted-foreground">en Odoo sin MercadoLibre</span>
                        </div>
                    </div>
                </SummaryCard>
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
                        placeholder="Buscar por SKU, nombre o número de parte"
                        aria-label="Buscar productos"
                        className="pl-8"
                    />
                </div>
                <Select
                    value={sourceId}
                    onValueChange={(value) => {
                        setSourceId(value ?? ALL)
                        setOffset(0)
                    }}
                    items={sourceItems}
                >
                    <SelectTrigger className="w-48" aria-label="Filtrar por source">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {sourceItems.map((s) => (
                                <SelectItem key={s.value} value={s.value}>
                                    {s.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <Select
                    value={channelCode}
                    onValueChange={(value) => {
                        setChannelCode(value ?? ALL)
                        setConnectionId(ALL)
                        setOffset(0)
                    }}
                    items={CHANNEL_ITEMS}
                >
                    <SelectTrigger className="w-48" aria-label="Filtrar por canal">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {CHANNEL_ITEMS.map((c) => (
                                <SelectItem key={c.value} value={c.value}>
                                    {c.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <Select
                    value={connectionId}
                    onValueChange={(value) => {
                        setConnectionId(value ?? ALL)
                        setOffset(0)
                    }}
                    items={connectionItems}
                >
                    <SelectTrigger className="w-64" aria-label="Filtrar por conexión">
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
                <Select
                    value={missingIn}
                    onValueChange={(value) => {
                        setMissingIn(value ?? ALL)
                        setOffset(0)
                    }}
                    items={missingItems}
                >
                    <SelectTrigger className="w-72" aria-label="Comparar faltantes">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {missingItems.map((m) => (
                                <SelectItem key={m.value} value={m.value}>
                                    {m.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <label className="flex items-center gap-2 text-sm">
                    <input
                        type="checkbox"
                        className="size-4"
                        checked={withStockAndPrice}
                        onChange={(e) => {
                            setWithStockAndPrice(e.target.checked)
                            setOffset(0)
                        }}
                    />
                    Solo con stock y precio
                </label>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
                Para comparar faltantes combina "canal / conexión" (dónde sí está publicado) con "Falta en" (dónde no).
            </p>

            {report.isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al cargar el reporte: {report.error instanceof Error ? report.error.message : "desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-3 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>SKU</TableHead>
                        <TableHead>Producto</TableHead>
                        <TableHead>N° de parte</TableHead>
                        <TableHead>Marca</TableHead>
                        <TableHead>Source</TableHead>
                        <TableHead>Odoo</TableHead>
                        <TableHead>MercadoLibre</TableHead>
                        <TableHead>Estado Odoo</TableHead>
                        <TableHead>Estado MercadoLibre</TableHead>
                        <TableHead>Atributos MercadoLibre</TableHead>
                        <TableHead className="text-right">Imágenes</TableHead>
                        <TableHead>Imágenes Marketing</TableHead>
                        <TableHead className="text-right">Stock</TableHead>
                        <TableHead className="text-right">Precio</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {report.isLoading && rows.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={14} className="text-center text-muted-foreground">
                                Cargando reporte...
                            </TableCell>
                        </TableRow>
                    ) : null}
                    {!report.isLoading && rows.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={14} className="text-center text-muted-foreground">
                                No hay productos publicados con estos filtros.
                            </TableCell>
                        </TableRow>
                    ) : null}
                    {rows.map((row) => (
                        <TableRow key={row.productId}>
                            <TableCell className="font-mono text-xs">
                                <ProductSKULink productId={row.productId} sku={row.sku} />
                            </TableCell>
                            <TableCell className="max-w-72">
                                <div className="truncate" title={row.name}>
                                    {row.name}
                                </div>
                            </TableCell>
                            <TableCell className="font-mono text-xs whitespace-nowrap">{row.partNumber}</TableCell>
                            <TableCell className="whitespace-nowrap">{row.brandName ?? "—"}</TableCell>
                            <TableCell className="whitespace-nowrap">{row.sourceName || "—"}</TableCell>
                            <TableCell className="align-top">
                                <YesNo value={liveConnections(row.odoo).length > 0} />
                                <ConnectionNames connections={liveConnections(row.odoo)} />
                            </TableCell>
                            <TableCell className="align-top">
                                <YesNo value={liveConnections(row.mercadoLibre).length > 0} />
                                <ConnectionNames connections={liveConnections(row.mercadoLibre)} />
                            </TableCell>
                            <TableCell className="align-top">
                                <ConnectionStates connections={row.odoo} />
                            </TableCell>
                            <TableCell className="align-top">
                                <ConnectionStates connections={row.mercadoLibre} />
                            </TableCell>
                            <TableCell className="align-top">
                                <AttributeStatuses statuses={row.meliAttributes} />
                            </TableCell>
                            <TableCell className="text-right tabular-nums">{row.imageCount}</TableCell>
                            <TableCell>
                                <YesNo value={row.hasMktImages} />
                            </TableCell>
                            <TableCell className="text-right tabular-nums">{row.stock.toLocaleString("es-MX")}</TableCell>
                            <TableCell className="text-right whitespace-nowrap tabular-nums">
                                {row.price ? formatPrice(row.price) : "—"}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={report.data?.total ?? 0}
                onOffsetChange={setOffset}
                onPageSizeChange={(size) => {
                    setPageSize(size)
                    setOffset(0)
                }}
                pageSizeOptions={[25, 50, 100, 200]}
            />
        </section>
    )
}

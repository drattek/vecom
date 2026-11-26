import { useState } from "react"
import { SearchIcon, TriangleAlertIcon } from "lucide-react"
import { Link, useSearchParams } from "react-router-dom"

import { PageHeader } from "@/components/PageHeader"
import { TablePagination } from "@/components/TablePagination"
import { SalesSyncPanel } from "@/components/sales/SalesSyncPanel"
import { StatusBadge } from "@/components/sales/StatusBadge"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { formatDateTime } from "@/lib/dashboard"
import {
    formatAmount,
    ORDER_STATUS,
    PAYMENT_STATUS,
    SHIPPING_STATUS,
    useSales,
    useSalesSyncStatus,
} from "@/lib/sales"

const ALL = "all"
const DEFAULT_PAGE_SIZE = 25
const COLUMNS = 9

type FilterItem = { value: string; label: string }

function statusItems(allLabel: string, statuses: Record<string, { label: string }>): FilterItem[] {
    return [
        { value: ALL, label: allLabel },
        ...Object.entries(statuses).map(([value, { label }]) => ({ value, label })),
    ]
}

function FilterSelect({ label, items, value, onChange }: {
    label: string
    items: FilterItem[]
    value: string
    onChange: (value: string) => void
}) {
    return (
        <Select items={items} value={value} onValueChange={(next) => onChange(String(next ?? ALL))}>
            <SelectTrigger aria-label={label}>
                <SelectValue />
            </SelectTrigger>
            <SelectContent>
                <SelectGroup>
                    {items.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                </SelectGroup>
            </SelectContent>
        </Select>
    )
}

export function SalesPage() {
    // Los filtros viven en la URL para conservarlos al volver del detalle.
    const [params, setParams] = useSearchParams()
    const read = (key: string) => params.get(key) ?? ""
    const offset = Math.max(0, Number(params.get("offset")) || 0)
    const pageSize = Number(params.get("pageSize")) || DEFAULT_PAGE_SIZE
    const [searchDraft, setSearchDraft] = useState(read("search"))

    const update = (changes: Record<string, string>) => {
        setParams((current) => {
            const next = new URLSearchParams(current)
            // Cambiar un filtro regresa a la primera página.
            if (!("offset" in changes)) next.delete("offset")
            for (const [key, value] of Object.entries(changes)) {
                if (value && value !== ALL) next.set(key, value)
                else next.delete(key)
            }
            return next
        }, { replace: true })
    }

    const syncStatus = useSalesSyncStatus()
    const connections = syncStatus.data?.connections ?? []
    const revision = connections.map((c) => c.lastFinishedAt ?? "").join("|")

    const sales = useSales(
        {
            channelCode: read("channelCode") || undefined,
            connectionId: read("connectionId") || undefined,
            orderStatus: read("orderStatus") || undefined,
            paymentStatus: read("paymentStatus") || undefined,
            shippingStatus: read("shippingStatus") || undefined,
            from: read("from") || undefined,
            to: read("to") || undefined,
            search: read("search") || undefined,
        },
        offset,
        pageSize,
        revision,
    )
    const rows = sales.data?.sales ?? []

    const channelItems: FilterItem[] = [
        { value: ALL, label: "Todos los canales" },
        ...[...new Map(connections.map((c) => [c.channelCode, c.channelName]))].map(([value, label]) => ({ value, label })),
    ]
    const connectionItems: FilterItem[] = [
        { value: ALL, label: "Todas las conexiones" },
        ...connections
            .filter((c) => !read("channelCode") || c.channelCode === read("channelCode"))
            .map((c) => ({ value: String(c.connectionId), label: `${c.connectionName} (${c.channelName})` })),
    ]

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Ventas</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <h2 className="text-xl font-semibold text-foreground">Ventas</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                    Ventas de MercadoLibre y de la tienda en línea de Odoo. Solo consulta.
                </p>

                {syncStatus.data ? (
                    <SalesSyncPanel
                        enabled={syncStatus.data.enabled}
                        intervalMinutes={syncStatus.data.intervalMinutes}
                        connections={connections}
                    />
                ) : null}
                {syncStatus.isError ? (
                    <p className="mt-2 text-sm text-destructive">
                        {getServerErrorMessage(syncStatus.error, "No se pudo consultar el estado de la sincronización.")}
                    </p>
                ) : null}

                <div className="mt-3 flex shrink-0 flex-wrap items-center gap-2">
                    <form
                        className="relative w-full sm:w-64"
                        onSubmit={(event) => {
                            event.preventDefault()
                            update({ search: searchDraft.trim() })
                        }}
                    >
                        <SearchIcon className="absolute top-1/2 left-2 size-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            className="pl-8"
                            aria-label="Buscar ventas"
                            placeholder="Orden, cliente o SKU (Enter)"
                            maxLength={100}
                            value={searchDraft}
                            onChange={(event) => setSearchDraft(event.target.value)}
                        />
                    </form>
                    <FilterSelect label="Canal" items={channelItems} value={read("channelCode") || ALL}
                        onChange={(value) => update({ channelCode: value, connectionId: "" })} />
                    <FilterSelect label="Conexión" items={connectionItems} value={read("connectionId") || ALL}
                        onChange={(value) => update({ connectionId: value })} />
                    <FilterSelect label="Estado de la venta" items={statusItems("Venta: todas", ORDER_STATUS)}
                        value={read("orderStatus") || ALL} onChange={(value) => update({ orderStatus: value })} />
                    <FilterSelect label="Estado del pago" items={statusItems("Pago: todos", PAYMENT_STATUS)}
                        value={read("paymentStatus") || ALL} onChange={(value) => update({ paymentStatus: value })} />
                    <FilterSelect label="Estado del envío" items={statusItems("Envío: todos", SHIPPING_STATUS)}
                        value={read("shippingStatus") || ALL} onChange={(value) => update({ shippingStatus: value })} />
                    <label className="flex items-center gap-1 text-sm text-muted-foreground">
                        Desde
                        <Input type="date" className="w-40" value={read("from")} max={read("to") || undefined}
                            onChange={(event) => update({ from: event.target.value })} />
                    </label>
                    <label className="flex items-center gap-1 text-sm text-muted-foreground">
                        Hasta
                        <Input type="date" className="w-40" value={read("to")} min={read("from") || undefined}
                            onChange={(event) => update({ to: event.target.value })} />
                    </label>
                </div>

                {sales.isError ? (
                    <p className="mt-2 text-sm text-destructive">
                        {getServerErrorMessage(sales.error, "No se pudieron consultar las ventas.")}
                    </p>
                ) : null}

                <Table containerClassName="mt-3 min-h-0 flex-1 overflow-auto">
                    <TableHeader>
                        <TableRow>
                            <TableHead>Fecha</TableHead>
                            <TableHead>Orden</TableHead>
                            <TableHead>Origen</TableHead>
                            <TableHead>Cliente</TableHead>
                            <TableHead className="text-right">Total</TableHead>
                            <TableHead>Venta</TableHead>
                            <TableHead>Pago</TableHead>
                            <TableHead>Envío</TableHead>
                            <TableHead>Despachar antes de</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {sales.isLoading ? (
                            <TableRow>
                                <TableCell colSpan={COLUMNS} className="text-center text-muted-foreground">
                                    Cargando ventas...
                                </TableCell>
                            </TableRow>
                        ) : null}
                        {sales.isSuccess && rows.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={COLUMNS} className="text-center text-muted-foreground">
                                    No se encontraron ventas.
                                </TableCell>
                            </TableRow>
                        ) : null}
                        {rows.map((sale) => (
                            <TableRow key={sale.id}>
                                <TableCell className="whitespace-nowrap">{formatDateTime(sale.orderedAt)}</TableCell>
                                <TableCell>
                                    <Link
                                        to={`/sales/${sale.id}`}
                                        state={{ listSearch: params.toString() }}
                                        className="font-medium text-primary underline-offset-2 hover:underline"
                                    >
                                        {sale.orderNumber ?? sale.externalId}
                                    </Link>
                                    <div className="text-xs text-muted-foreground">
                                        {sale.itemCount} {sale.itemCount === 1 ? "producto" : "productos"}
                                        {sale.unlinkedItems > 0 ? ` · ${sale.unlinkedItems} sin vincular` : ""}
                                    </div>
                                    {sale.isIncomplete ? (
                                        <div className="flex items-center gap-1 text-xs text-amber-700 dark:text-amber-400">
                                            <TriangleAlertIcon className="size-3" /> Datos incompletos
                                        </div>
                                    ) : null}
                                </TableCell>
                                <TableCell>
                                    {sale.channelName}
                                    <div className="text-xs text-muted-foreground">{sale.connectionName}</div>
                                </TableCell>
                                <TableCell>{sale.customerName ?? "—"}</TableCell>
                                <TableCell className="text-right whitespace-nowrap tabular-nums">
                                    {formatAmount(sale.totalAmount, sale.currencyCode)}
                                </TableCell>
                                <TableCell><StatusBadge statuses={ORDER_STATUS} value={sale.orderStatus} /></TableCell>
                                <TableCell><StatusBadge statuses={PAYMENT_STATUS} value={sale.paymentStatus} /></TableCell>
                                <TableCell><StatusBadge statuses={SHIPPING_STATUS} value={sale.shippingStatus} /></TableCell>
                                <TableCell className="whitespace-nowrap text-muted-foreground">
                                    {["pending", "ready_to_ship"].includes(sale.shippingStatus) && sale.orderStatus !== "cancelled"
                                        ? formatDateTime(sale.dispatchDeadline)
                                        : "—"}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>

                <TablePagination
                    offset={offset}
                    pageSize={pageSize}
                    total={sales.data?.total ?? 0}
                    onOffsetChange={(next) => update({ offset: next > 0 ? String(next) : "" })}
                    onPageSizeChange={(next) => update({ pageSize: String(next) })}
                />
            </section>
        </main>
    )
}

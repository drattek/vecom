import { useState } from "react"
import { CheckIcon, XIcon } from "lucide-react"

import { BlockError, DashboardTable } from "@/components/dashboard/DashboardTable"
import { ProductCell } from "@/components/dashboard/ProductCell"
import { TablePagination } from "@/components/TablePagination"
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { TableCell, TableRow } from "@/components/ui/table"
import { useCatalogCount, useCatalogProducts, useConnectionsHealth } from "@/lib/dashboard"
import type { CatalogFilter } from "@/lib/schemas/dashboard"

const ALL = "all"

type FilterConfig = {
    filter: CatalogFilter
    label: string
    /** Solo tiene sentido contra una conexión concreta. */
    needsConnection?: boolean
}

const FILTERS: FilterConfig[] = [
    { filter: "no_image", label: "Sin imagen de portada" },
    { filter: "no_price", label: "Sin precio vigente" },
    { filter: "no_brand", label: "Sin marca" },
    { filter: "no_category", label: "Sin categoría" },
    { filter: "no_stock", label: "Sin stock" },
    { filter: "category_unmapped", label: "Categoría sin mapear", needsConnection: true },
    { filter: "ready_unpublished", label: "Listos sin publicar" },
]

const COLUMNS = ["Producto", "Marca", "Categoría", "Imagen", "Precio", "Stock"]

function Requirement({ met }: { met: boolean }) {
    return met ? (
        <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 dark:text-emerald-400">
            <CheckIcon className="size-3.5" /> Sí
        </span>
    ) : (
        <span className="inline-flex items-center gap-1 text-xs font-medium text-destructive">
            <XIcon className="size-3.5" /> No
        </span>
    )
}

function FilterCard({
    config,
    connectionId,
    selected,
    onSelect,
}: {
    config: FilterConfig
    connectionId: number | null
    selected: boolean
    onSelect: () => void
}) {
    const disabled = Boolean(config.needsConnection) && connectionId === null
    const query = useCatalogCount(config.filter, connectionId, !disabled)

    let value
    if (disabled) {
        value = <span className="block text-xs text-muted-foreground">Elige una conexión</span>
    } else if (query.isPending) {
        value = <Skeleton className="mt-1 h-8 w-16" />
    } else {
        value = (
            <span className="block text-2xl font-semibold tabular-nums">
                {query.isError ? "—" : query.data.toLocaleString("es-MX")}
            </span>
        )
    }

    return (
        <button
            type="button"
            aria-pressed={selected}
            disabled={disabled}
            onClick={onSelect}
            className={`rounded-lg border p-3 text-left outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-60 ${
                selected ? "border-primary bg-muted/50 ring-1 ring-primary" : "border-border"
            }`}
        >
            <span className="block text-xs text-muted-foreground">{config.label}</span>
            {value}
        </button>
    )
}

/** Tabla del filtro activo. Se remonta (key) al cambiar filtro o conexión para volver a la página 1. */
function CatalogTable({ filter, connectionId }: { filter: CatalogFilter; connectionId: number | null }) {
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(10)
    // El total comparte consulta con la card; la tabla no espera por él.
    const count = useCatalogCount(filter, connectionId)
    const query = useCatalogProducts(filter, connectionId, offset, pageSize)
    const rows = query.data?.rows ?? []

    if (query.isError) {
        return <BlockError error={query.error} onRetry={() => void query.refetch()} />
    }

    return (
        <>
            <DashboardTable
                columns={COLUMNS}
                isLoading={query.isPending}
                isStale={query.isPlaceholderData}
                isEmpty={rows.length === 0}
                emptyMessage="No hay productos en este filtro."
            >
                {rows.map((row) => (
                    <TableRow key={row.productId}>
                        <TableCell>
                            <ProductCell productId={row.productId} sku={row.sku} name={row.productName} />
                        </TableCell>
                        <TableCell>{row.hasBrand ? (row.brandName ?? "—") : <Requirement met={false} />}</TableCell>
                        <TableCell>
                            {row.hasCategory ? (row.categoryName ?? "—") : <Requirement met={false} />}
                        </TableCell>
                        <TableCell>
                            <Requirement met={row.hasImage} />
                        </TableCell>
                        <TableCell>
                            <Requirement met={row.hasPrice} />
                        </TableCell>
                        <TableCell>
                            <Requirement met={row.hasStock} />
                        </TableCell>
                    </TableRow>
                ))}
            </DashboardTable>
            {count.isError ? (
                <BlockError error={count.error} onRetry={() => void count.refetch()} />
            ) : count.data !== undefined && count.data > 10 ? (
                <TablePagination
                    offset={offset}
                    pageSize={pageSize}
                    total={count.data}
                    onOffsetChange={setOffset}
                    onPageSizeChange={(size) => {
                        setPageSize(size)
                        setOffset(0)
                    }}
                />
            ) : null}
        </>
    )
}

export function DashboardCatalogPage() {
    const [connectionId, setConnectionId] = useState<number | null>(null)
    const [filter, setFilter] = useState<CatalogFilter>("ready_unpublished")
    const connections = useConnectionsHealth()

    const connectionItems = [
        { value: ALL, label: "Todas las conexiones" },
        ...(connections.data ?? [])
            .filter((connection) => connection.status === "active")
            .map((connection) => ({
                value: String(connection.connectionId),
                label: `${connection.connectionName} (${connection.channelName})`,
            })),
    ]

    const handleConnectionChange = (value: string | null) => {
        const next = !value || value === ALL ? null : Number(value)
        setConnectionId(next)
        if (next === null && filter === "category_unmapped") {
            setFilter("ready_unpublished")
        }
    }

    const activeLabel = FILTERS.find((item) => item.filter === filter)?.label

    return (
        <>
            <h2 className="text-xl font-semibold text-foreground">Preparación del catálogo</h2>
            <p className="mt-1 text-sm text-muted-foreground">
                Por qué hay productos que no se están vendiendo. Un producto puede aparecer en varios filtros, así que
                las cifras no se suman.
            </p>

            <div className="mt-4 flex flex-wrap items-center gap-2">
                <Select
                    value={connectionId === null ? ALL : String(connectionId)}
                    onValueChange={handleConnectionChange}
                    items={connectionItems}
                >
                    <SelectTrigger className="w-80" aria-label="Filtrar por conexión">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {connectionItems.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                    {item.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                    {connectionId === null
                        ? "Incluye todos los productos activos y vendibles."
                        : "Incluye solo los productos activos y vendibles sin publicación en esta conexión."}
                </p>
            </div>
            {connections.isError ? (
                <div className="mt-2">
                    <BlockError error={connections.error} onRetry={() => void connections.refetch()} />
                </div>
            ) : null}

            <div className="mt-4 grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-7">
                {FILTERS.map((config) => (
                    <FilterCard
                        key={config.filter}
                        config={config}
                        connectionId={connectionId}
                        selected={filter === config.filter}
                        onSelect={() => setFilter(config.filter)}
                    />
                ))}
            </div>

            <section className="mt-4 rounded-xl border bg-card p-4">
                <h3 className="font-semibold text-foreground">{activeLabel}</h3>
                <p className="mb-3 text-sm text-muted-foreground">
                    {filter === "ready_unpublished"
                        ? "Tienen portada, precio base vigente, marca y categoría mapeada, y falta su publicación en al menos una conexión que acepta su stock. No se evalúan el tipo de cambio, la fórmula de precio ni los atributos que exige el canal."
                        : filter === "no_stock"
                          ? "Odoo publica sin existencias; en los demás canales el producto espera a tener stock."
                          : "Las columnas muestran todo lo que le falta a cada producto."}
                </p>
                <CatalogTable key={`${filter}-${connectionId}`} filter={filter} connectionId={connectionId} />
            </section>
        </>
    )
}

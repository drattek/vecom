import {
    Badge,
    SectionCard,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
import { TablePagination } from "@/components/TablePagination"
import { formatAmount, formatDate, useProductSection, useProductSectionPage } from "@/lib/product-detail"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"
import {
    paginatedProductPriceHistorySchema,
    productPricingSectionSchema,
} from "@/lib/schemas/product-details"
import { TrendingDownIcon, TrendingUpIcon } from "lucide-react"
import { useState } from "react"
import { useParams } from "react-router-dom"

const priceListStatusLabels: Record<string, string> = {
    active: "Activa",
    discontinued: "Descontinuada",
    hidden: "Oculta",
}

export function ProductPricingSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "pricing", productPricingSectionSchema)

    const [historyPage, setHistoryPage] = useState({ offset: 0, pageSize: 10 })
    const history = useProductSectionPage(
        productId,
        "pricing/history",
        paginatedProductPriceHistorySchema,
        historyPage,
    )
    const historyRows = history.data?.data ?? []
    const historyTotal = history.data?.total ?? 0

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudieron cargar los precios del producto." />
    }

    const effective = data.current.find((price) => price.isEffective)

    return (
        <div className="flex flex-col gap-4">
            <SectionCard
                title="Precio vigente"
                description="La lista activa de mayor prioridad cuya vigencia cubre el día de hoy."
            >
                {effective ? (
                    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                        <span className="text-2xl font-semibold tabular-nums text-foreground">
                            {formatAmount(effective.price, effective.currencySymbol, effective.currencyCode)}
                        </span>
                        <span className="text-sm text-muted-foreground">{effective.priceListName}</span>
                        {effective.taxIncluded ? <Badge>Impuesto incluido</Badge> : null}
                    </div>
                ) : (
                    <SectionState state="empty" message="El producto no tiene un precio vigente en este momento." />
                )}
            </SectionCard>

            <SectionCard
                title="Precios por lista"
                action={<span className="text-xs text-muted-foreground">{data.current.length} listas</span>}
            >
                {data.current.length === 0 ? (
                    <SectionState state="empty" message="Este producto no tiene precios registrados." />
                ) : (
                    <Table containerClassName="overflow-x-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Lista</TableHead>
                                <TableHead>Precio</TableHead>
                                <TableHead>Margen</TableHead>
                                <TableHead>Prioridad</TableHead>
                                <TableHead>Estatus</TableHead>
                                <TableHead>Vigencia</TableHead>
                                <TableHead>Actualizado</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {data.current.map((price) => (
                                <TableRow key={price.id} className={cn(price.isEffective && "bg-muted/40")}>
                                    <TableCell>
                                        <div className="flex items-center gap-2">
                                            <span>{price.priceListName}</span>
                                            {price.isEffective ? <Badge tone="success">Vigente</Badge> : null}
                                        </div>
                                    </TableCell>
                                    <TableCell className="tabular-nums whitespace-nowrap">
                                        {formatAmount(price.price, price.currencySymbol, price.currencyCode)}
                                    </TableCell>
                                    <TableCell className="tabular-nums text-muted-foreground">{price.margin}</TableCell>
                                    <TableCell className="tabular-nums text-muted-foreground">
                                        {price.priceListPriority}
                                    </TableCell>
                                    <TableCell>
                                        <Badge tone={price.status === "active" ? "success" : "muted"}>
                                            {priceListStatusLabels[price.status] ?? price.status}
                                        </Badge>
                                    </TableCell>
                                    <TableCell className="whitespace-nowrap text-muted-foreground">
                                        {formatDate(price.validFrom)} — {formatDate(price.validTo)}
                                    </TableCell>
                                    <TableCell className="whitespace-nowrap text-muted-foreground">
                                        {formatDate(price.updatedAt, true)}
                                        {price.updatedBy ? ` · ${price.updatedBy}` : ""}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </SectionCard>

            <SectionCard
                title="Historial de cambios"
                description="Cambios de precio registrados, del más reciente al más antiguo."
                action={
                    <span className="text-xs text-muted-foreground">
                        {historyTotal.toLocaleString("es-MX")} movimientos
                    </span>
                }
            >
                {history.isError ? (
                    <SectionState state="error" message="No se pudo cargar el historial de cambios." />
                ) : history.isLoading && historyRows.length === 0 ? (
                    <SectionState state="loading" />
                ) : historyTotal === 0 ? (
                    <SectionState state="empty" message="No hay cambios de precio registrados." />
                ) : (
                    <div className="flex flex-col gap-3">
                        <Table containerClassName="overflow-x-auto">
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Fecha</TableHead>
                                    <TableHead>Lista</TableHead>
                                    <TableHead>Anterior</TableHead>
                                    <TableHead>Nuevo</TableHead>
                                    <TableHead>Cambio</TableHead>
                                    <TableHead>Por</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {historyRows.map((entry) => {
                                    const oldPrice = Number(entry.oldPrice)
                                    const newPrice = Number(entry.newPrice)
                                    const difference = newPrice - oldPrice
                                    const isUp = difference > 0
                                    const hasChange = Number.isFinite(difference) && difference !== 0

                                    return (
                                        <TableRow key={entry.id}>
                                            <TableCell className="whitespace-nowrap text-muted-foreground">
                                                {formatDate(entry.createdAt, true)}
                                            </TableCell>
                                            <TableCell>{entry.priceListName ?? "—"}</TableCell>
                                            <TableCell className="tabular-nums whitespace-nowrap text-muted-foreground">
                                                {formatAmount(entry.oldPrice, entry.currencySymbol, entry.currencyCode)}
                                            </TableCell>
                                            <TableCell className="tabular-nums whitespace-nowrap">
                                                {formatAmount(entry.newPrice, entry.currencySymbol, entry.currencyCode)}
                                            </TableCell>
                                            <TableCell>
                                                {hasChange ? (
                                                    <span
                                                        className={cn(
                                                            "inline-flex items-center gap-1 tabular-nums whitespace-nowrap text-sm",
                                                            isUp
                                                                ? "text-emerald-700 dark:text-emerald-400"
                                                                : "text-destructive",
                                                        )}
                                                    >
                                                        {isUp ? (
                                                            <TrendingUpIcon className="size-3.5" />
                                                        ) : (
                                                            <TrendingDownIcon className="size-3.5" />
                                                        )}
                                                        {isUp ? "+" : ""}
                                                        {difference.toLocaleString("es-MX", {
                                                            minimumFractionDigits: 2,
                                                            maximumFractionDigits: 2,
                                                        })}
                                                    </span>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">{entry.updatedBy ?? "—"}</TableCell>
                                        </TableRow>
                                    )
                                })}
                            </TableBody>
                        </Table>
                        <TablePagination
                            offset={historyPage.offset}
                            pageSize={historyPage.pageSize}
                            total={historyTotal}
                            onOffsetChange={(offset) => setHistoryPage((prev) => ({ ...prev, offset }))}
                            onPageSizeChange={(pageSize) => setHistoryPage({ offset: 0, pageSize })}
                        />
                    </div>
                )}
            </SectionCard>
        </div>
    )
}

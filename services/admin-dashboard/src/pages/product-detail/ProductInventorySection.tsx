import {
    Badge,
    SectionCard,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
import { TablePagination } from "@/components/TablePagination"
import { formatDate, useProductSection, useProductSectionPage } from "@/lib/product-detail"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"
import {
    paginatedProductStockMovementsSchema,
    productInventorySectionSchema,
} from "@/lib/schemas/product-details"
import { useState } from "react"
import { useParams } from "react-router-dom"

const movementTypeLabels: Record<string, string> = {
    in: "Entrada",
    out: "Salida",
    adjustment: "Ajuste",
    transfer_in: "Traspaso entrada",
    transfer_out: "Traspaso salida",
    sync: "Sincronización",
}

export function ProductInventorySection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "inventory", productInventorySectionSchema)

    const [movementsPage, setMovementsPage] = useState({ offset: 0, pageSize: 10 })
    const movements = useProductSectionPage(
        productId,
        "inventory/movements",
        paginatedProductStockMovementsSchema,
        movementsPage,
    )
    const movementRows = movements.data?.data ?? []
    const movementsTotal = movements.data?.total ?? 0

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudo cargar el inventario del producto." />
    }

    return (
        <div className="flex flex-col gap-4">
            <SectionCard title="Stock total" description="Suma de las existencias disponibles en todos los almacenes.">
                <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                    <span className="text-2xl font-semibold tabular-nums text-foreground">
                        {data.totalStock.toLocaleString("es-MX")}
                    </span>
                    <span className="text-sm text-muted-foreground">
                        {data.byLocation.length === 1
                            ? "en 1 almacén"
                            : `distribuido en ${data.byLocation.length} almacenes`}
                    </span>
                </div>
            </SectionCard>

            <SectionCard
                title="Existencias por almacén"
                action={<span className="text-xs text-muted-foreground">{data.byLocation.length} ubicaciones</span>}
            >
                {data.byLocation.length === 0 ? (
                    <SectionState state="empty" message="Este producto no tiene existencias registradas." />
                ) : (
                    <Table containerClassName="overflow-x-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Sucursal</TableHead>
                                <TableHead>Almacén</TableHead>
                                <TableHead>Disponible</TableHead>
                                <TableHead>Última sincronización</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {data.byLocation.map((location) => (
                                <TableRow key={location.id}>
                                    <TableCell>{location.branchName}</TableCell>
                                    <TableCell>{location.warehouseName}</TableCell>
                                    <TableCell className="tabular-nums">
                                        <span className={cn(location.availableQty === 0 && "text-muted-foreground")}>
                                            {location.availableQty.toLocaleString("es-MX")}
                                        </span>
                                    </TableCell>
                                    <TableCell className="whitespace-nowrap text-muted-foreground">
                                        {formatDate(location.lastSyncAt, true)}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </SectionCard>

            <SectionCard
                title="Historial de movimientos"
                description="Movimientos de inventario registrados, del más reciente al más antiguo."
                action={
                    <span className="text-xs text-muted-foreground">
                        {movementsTotal.toLocaleString("es-MX")} movimientos
                    </span>
                }
            >
                {movements.isError ? (
                    <SectionState state="error" message="No se pudo cargar el historial de movimientos." />
                ) : movements.isLoading && movementRows.length === 0 ? (
                    <SectionState state="loading" />
                ) : movementsTotal === 0 ? (
                    <SectionState state="empty" message="No hay movimientos de inventario registrados." />
                ) : (
                    <div className="flex flex-col gap-3">
                        <Table containerClassName="overflow-x-auto">
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Fecha</TableHead>
                                    <TableHead>Tipo</TableHead>
                                    <TableHead>Sucursal</TableHead>
                                    <TableHead>Almacén</TableHead>
                                    <TableHead>Antes</TableHead>
                                    <TableHead>Cambio</TableHead>
                                    <TableHead>Después</TableHead>
                                    <TableHead>Por</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {movementRows.map((movement) => (
                                    <TableRow key={movement.id}>
                                        <TableCell className="whitespace-nowrap text-muted-foreground">
                                            {formatDate(movement.createdAt, true)}
                                        </TableCell>
                                        <TableCell>
                                            <Badge tone={movement.movementType === "sync" ? "muted" : "neutral"}>
                                                {movementTypeLabels[movement.movementType] ?? movement.movementType}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>{movement.branchName ?? "—"}</TableCell>
                                        <TableCell>{movement.warehouseName ?? "—"}</TableCell>
                                        <TableCell className="tabular-nums text-muted-foreground">
                                            {movement.quantityBefore.toLocaleString("es-MX")}
                                        </TableCell>
                                        <TableCell
                                            className={cn(
                                                "tabular-nums whitespace-nowrap",
                                                movement.quantityChange > 0 && "text-emerald-700 dark:text-emerald-400",
                                                movement.quantityChange < 0 && "text-destructive",
                                                movement.quantityChange === 0 && "text-muted-foreground",
                                            )}
                                        >
                                            {movement.quantityChange > 0 ? "+" : ""}
                                            {movement.quantityChange.toLocaleString("es-MX")}
                                        </TableCell>
                                        <TableCell className="tabular-nums">
                                            {movement.quantityAfter.toLocaleString("es-MX")}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {movement.updatedBy ?? "—"}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                        <TablePagination
                            offset={movementsPage.offset}
                            pageSize={movementsPage.pageSize}
                            total={movementsTotal}
                            onOffsetChange={(offset) => setMovementsPage((prev) => ({ ...prev, offset }))}
                            onPageSizeChange={(pageSize) => setMovementsPage({ offset: 0, pageSize })}
                        />
                    </div>
                )}
            </SectionCard>
        </div>
    )
}

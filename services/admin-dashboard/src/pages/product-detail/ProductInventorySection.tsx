import {
    Badge,
    FieldError,
    SectionCard,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
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
import { getServerErrorMessage } from "@/lib/api"
import {
    formatDate,
    useCreateStockMovement,
    useProductSection,
    useProductSectionPage,
    useStockWarehouseOptions,
    type ManualStockMovementType,
} from "@/lib/product-detail"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"
import {
    paginatedProductStockMovementsSchema,
    productInventorySectionSchema,
    type ProductStockLocation,
} from "@/lib/schemas/product-details"
import { PlusIcon } from "lucide-react"
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

const manualTypeOptions: { value: ManualStockMovementType; label: string }[] = [
    { value: "in", label: "Entrada (suma piezas)" },
    { value: "out", label: "Salida (resta piezas)" },
    { value: "adjustment", label: "Ajuste (fija la cantidad)" },
]

const NO_WAREHOUSE = ""

/**
 * StockMovementForm registra un movimiento manual sobre una ubicación del
 * producto. Entrada/Salida indican cuántas piezas se suman o restan; Ajuste
 * indica la cantidad final que debe quedar. El core valida de nuevo y aplica
 * stock + movimiento en una transacción; aquí solo se evita mandar lo obvio
 * (sin ubicación, cantidad inválida) y se muestra cómo quedaría el stock.
 */
function StockMovementForm({
    productId,
    locations,
    onDone,
}: {
    productId?: string
    locations: ProductStockLocation[]
    onDone: () => void
}) {
    const warehouses = useStockWarehouseOptions()
    const createMovement = useCreateStockMovement(productId)

    const [warehouseId, setWarehouseId] = useState(
        locations.length === 1 ? String(locations[0].warehouseId) : NO_WAREHOUSE,
    )
    const [movementType, setMovementType] = useState<ManualStockMovementType>("adjustment")
    const [quantity, setQuantity] = useState("")
    const [error, setError] = useState<string | null>(null)

    const warehouseItems = (warehouses.data ?? []).map((warehouse) => ({
        value: String(warehouse.id),
        label: `${warehouse.branchName} · ${warehouse.name}`,
    }))

    const selectedWarehouse = warehouses.data?.find((warehouse) => String(warehouse.id) === warehouseId)
    const currentQty =
        locations.find((location) => String(location.warehouseId) === warehouseId)?.availableQty ?? 0

    const parsedQuantity = quantity.trim() === "" ? null : Number(quantity)
    const hasValidQuantity = parsedQuantity != null && Number.isInteger(parsedQuantity)
    const resultingQty = !hasValidQuantity
        ? null
        : movementType === "in"
          ? currentQty + parsedQuantity
          : movementType === "out"
            ? currentQty - parsedQuantity
            : parsedQuantity

    function submit() {
        if (!selectedWarehouse) {
            setError("Elegí la sucursal y el almacén.")
            return
        }
        if (!hasValidQuantity || parsedQuantity == null) {
            setError("Ingresá una cantidad entera.")
            return
        }
        if (movementType === "adjustment" ? parsedQuantity < 0 : parsedQuantity <= 0) {
            setError(movementType === "adjustment" ? "La cantidad final no puede ser negativa." : "La cantidad debe ser mayor a 0.")
            return
        }
        if (resultingQty != null && resultingQty < 0) {
            setError(`No hay suficiente stock: hay ${currentQty.toLocaleString("es-MX")} disponibles.`)
            return
        }
        if (resultingQty === currentQty) {
            setError("El movimiento no cambia el stock actual.")
            return
        }

        setError(null)
        createMovement.mutate(
            {
                branchId: selectedWarehouse.branchId,
                warehouseId: selectedWarehouse.id,
                movementType,
                quantity: parsedQuantity,
            },
            {
                onSuccess: onDone,
                onError: (mutationError) =>
                    setError(getServerErrorMessage(mutationError, "No se pudo registrar el movimiento.")),
            },
        )
    }

    const isBusy = createMovement.isPending

    return (
        <div className="mb-4 flex flex-col gap-2 rounded-md border border-border bg-muted/30 p-3">
            <div className="flex flex-wrap items-center gap-2">
                <Select
                    value={warehouseId}
                    onValueChange={(value) => setWarehouseId(value ?? NO_WAREHOUSE)}
                    items={warehouseItems}
                    disabled={isBusy || warehouses.isLoading}
                >
                    <SelectTrigger className="w-64" aria-label="Sucursal y almacén">
                        <SelectValue placeholder="Sucursal · Almacén" />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {warehouseItems.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                    {item.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <Select
                    value={movementType}
                    onValueChange={(value) => setMovementType((value as ManualStockMovementType) ?? "adjustment")}
                    items={manualTypeOptions}
                    disabled={isBusy}
                >
                    <SelectTrigger className="w-56" aria-label="Tipo de movimiento">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {manualTypeOptions.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                    {option.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                <Input
                    type="number"
                    inputMode="numeric"
                    min={0}
                    step={1}
                    value={quantity}
                    onChange={(event) => setQuantity(event.target.value)}
                    placeholder={movementType === "adjustment" ? "Cantidad final" : "Piezas"}
                    aria-label={movementType === "adjustment" ? "Cantidad final" : "Piezas"}
                    disabled={isBusy}
                    className="w-32"
                    autoComplete="off"
                    onKeyDown={(event) => {
                        if (event.key === "Enter") {
                            event.preventDefault()
                            submit()
                        }
                    }}
                />
                <Button size="sm" onClick={submit} disabled={isBusy}>
                    {isBusy ? "Guardando..." : "Registrar"}
                </Button>
                <Button size="sm" variant="ghost" onClick={onDone} disabled={isBusy}>
                    Cancelar
                </Button>
            </div>
            {selectedWarehouse ? (
                <p className="text-xs text-muted-foreground">
                    Stock actual en esta ubicación:{" "}
                    <span className="tabular-nums text-foreground">{currentQty.toLocaleString("es-MX")}</span>
                    {resultingQty != null ? (
                        <>
                            {" "}
                            → quedaría en{" "}
                            <span
                                className={cn(
                                    "tabular-nums text-foreground",
                                    resultingQty < 0 && "text-destructive",
                                )}
                            >
                                {resultingQty.toLocaleString("es-MX")}
                            </span>
                        </>
                    ) : null}
                </p>
            ) : null}
            {warehouses.isError ? (
                <FieldError message="No se pudieron cargar las sucursales y almacenes." />
            ) : null}
            <FieldError message={error} />
        </div>
    )
}

export function ProductInventorySection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "inventory", productInventorySectionSchema)
    const [isMovementFormOpen, setIsMovementFormOpen] = useState(false)

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
                action={
                    <div className="flex items-center gap-3">
                        <span className="text-xs text-muted-foreground">{data.byLocation.length} ubicaciones</span>
                        {isMovementFormOpen ? null : (
                            <Button variant="ghost" size="sm" onClick={() => setIsMovementFormOpen(true)}>
                                <PlusIcon />
                                Registrar movimiento
                            </Button>
                        )}
                    </div>
                }
            >
                {isMovementFormOpen ? (
                    <StockMovementForm
                        productId={productId}
                        locations={data.byLocation}
                        onDone={() => setIsMovementFormOpen(false)}
                    />
                ) : null}
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

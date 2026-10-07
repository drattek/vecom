import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { VehicleFitmentDialog } from "@/components/fitments/FitmentDialogs"
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useDebouncedValue } from "@/hooks/use-debounced-value"
import { getServerErrorMessage } from "@/lib/api"
import { formatProductCount, formatYearRange, useDeleteVehicleFitment, useVehicleFitments } from "@/lib/fitments"
import { formatDate, useBrandOptions } from "@/lib/product-detail"
import type { VehicleFitment } from "@/lib/schemas/fitments"
import { PencilIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

const ALL_BRANDS = "0"

export function FitmentsVehiclesPage() {
    const [query, setQuery] = useState("")
    const [brandId, setBrandId] = useState(ALL_BRANDS)
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(10)
    const [notice, setNotice] = useState<string | null>(null)

    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<VehicleFitment | null>(null)
    const [deleting, setDeleting] = useState<VehicleFitment | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    const debouncedQuery = useDebouncedValue(query)
    const brands = useBrandOptions()
    const { data, isLoading, isError, error } = useVehicleFitments({
        q: debouncedQuery,
        brandId: brandId === ALL_BRANDS ? null : Number(brandId),
        offset,
        pageSize,
    })
    const deleteMutation = useDeleteVehicleFitment()

    const fitments = data?.fitments ?? []
    const total = data?.total ?? 0

    const brandItems = [
        { label: "Todas las marcas", value: ALL_BRANDS },
        ...(brands.data ?? []).map((brand) => ({ label: brand.name, value: String(brand.id) })),
    ]

    function openEditor(fitment: VehicleFitment | null) {
        setEditing(fitment)
        setEditorOpen(true)
    }

    function askDelete(fitment: VehicleFitment) {
        setDeleteError(null)
        setDeleting(fitment)
    }

    async function confirmDelete() {
        if (!deleting) {
            return
        }
        setDeleteError(null)
        try {
            // Con productos vinculados el usuario ya vio el aviso en el diálogo:
            // confirmar equivale a desvincularlos (force).
            await deleteMutation.mutateAsync({ id: deleting.id, force: deleting.productCount > 0 })
            setNotice("Vehículo eliminado.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar el vehículo."))
        }
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Vehículos</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Catálogo de vehículos (marca, modelo y años) a los que se vinculan las compatibilidades de los productos.
                    </p>
                </div>
                <Button size="sm" onClick={() => openEditor(null)}>
                    <PlusIcon />
                    Nuevo vehículo
                </Button>
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-2">
                <div className="relative w-full max-w-xs">
                    <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={query}
                        onChange={(event) => {
                            setQuery(event.target.value)
                            setOffset(0)
                        }}
                        placeholder="Buscar por marca, modelo o año"
                        aria-label="Buscar vehículos"
                        className="pl-8"
                    />
                </div>
                <Select
                    value={brandId}
                    onValueChange={(value) => {
                        setBrandId(value ?? ALL_BRANDS)
                        setOffset(0)
                    }}
                    items={brandItems}
                    disabled={brands.isLoading}
                >
                    <SelectTrigger className="w-48" aria-label="Filtrar por marca">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {brandItems.map((brand) => (
                                <SelectItem key={brand.value} value={brand.value}>
                                    {brand.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los vehículos: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Marca</TableHead>
                        <TableHead>Modelo</TableHead>
                        <TableHead>Años</TableHead>
                        <TableHead>Productos</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && fitments.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                Cargando vehículos...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && fitments.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={6} className="text-center text-muted-foreground">
                                {debouncedQuery.trim() || brandId !== ALL_BRANDS
                                    ? "Ningún vehículo coincide con la búsqueda."
                                    : "Aún no hay vehículos. Crea el primero con «Nuevo vehículo»."}
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {fitments.map((fitment) => (
                        <TableRow key={fitment.id}>
                            <TableCell>{fitment.brandName || "—"}</TableCell>
                            <TableCell>{fitment.model}</TableCell>
                            <TableCell className="whitespace-nowrap">
                                {formatYearRange(fitment.yearStart, fitment.yearEnd)}
                            </TableCell>
                            <TableCell className={fitment.productCount === 0 ? "text-muted-foreground" : ""}>
                                {fitment.productCount}
                            </TableCell>
                            <TableCell className="whitespace-nowrap text-muted-foreground">
                                {formatDate(fitment.updatedAt, true)}
                            </TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Editar ${fitment.brandName} ${fitment.model}`}
                                        onClick={() => openEditor(fitment)}
                                    >
                                        <PencilIcon />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Eliminar ${fitment.brandName} ${fitment.model}`}
                                        onClick={() => askDelete(fitment)}
                                    >
                                        <Trash2Icon />
                                    </Button>
                                </div>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

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

            <VehicleFitmentDialog open={editorOpen} onOpenChange={setEditorOpen} item={editing} onSaved={setNotice} />

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar vehículo"
                description={
                    deleting ? (
                        <>
                            Se eliminará <strong>{deleting.brandName} {deleting.model}</strong> (
                            {formatYearRange(deleting.yearStart, deleting.yearEnd)}).
                            {deleting.productCount > 0
                                ? ` Está vinculado a ${formatProductCount(deleting.productCount)}: también se les quitará esta compatibilidad.`
                                : ""}
                        </>
                    ) : null
                }
                pending={deleteMutation.isPending}
                error={deleteError}
                onConfirm={() => void confirmDelete()}
            />
        </section>
    )
}

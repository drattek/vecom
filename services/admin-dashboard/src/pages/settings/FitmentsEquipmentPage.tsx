import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { EquipmentFitmentDialog } from "@/components/fitments/FitmentDialogs"
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
import {
    formatProductCount,
    useDeleteEquipmentFitment,
    useEquipmentFitments,
    useEquipmentTypes,
} from "@/lib/fitments"
import { formatDate, useBrandOptions } from "@/lib/product-detail"
import type { EquipmentFitment } from "@/lib/schemas/fitments"
import { PencilIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

const ALL = "0"

function fitmentLabel(fitment: EquipmentFitment): string {
    return [fitment.brandName, fitment.equipmentTypeName, fitment.model, fitment.serie].filter(Boolean).join(" · ")
}

export function FitmentsEquipmentPage() {
    const [query, setQuery] = useState("")
    const [brandId, setBrandId] = useState(ALL)
    const [typeId, setTypeId] = useState(ALL)
    const [offset, setOffset] = useState(0)
    const [pageSize, setPageSize] = useState(10)
    const [notice, setNotice] = useState<string | null>(null)

    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<EquipmentFitment | null>(null)
    const [deleting, setDeleting] = useState<EquipmentFitment | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    const debouncedQuery = useDebouncedValue(query)
    const brands = useBrandOptions()
    const types = useEquipmentTypes()
    const { data, isLoading, isError, error } = useEquipmentFitments({
        q: debouncedQuery,
        brandId: brandId === ALL ? null : Number(brandId),
        equipmentTypeId: typeId === ALL ? null : Number(typeId),
        offset,
        pageSize,
    })
    const deleteMutation = useDeleteEquipmentFitment()

    const fitments = data?.fitments ?? []
    const total = data?.total ?? 0

    const brandItems = [
        { label: "Todas las marcas", value: ALL },
        ...(brands.data ?? []).map((brand) => ({ label: brand.name, value: String(brand.id) })),
    ]
    const typeItems = [
        { label: "Todos los tipos", value: ALL },
        ...(types.data ?? []).map((type) => ({ label: type.name, value: String(type.id) })),
    ]

    function openEditor(fitment: EquipmentFitment | null) {
        setEditing(fitment)
        setEditorOpen(true)
    }

    function askDelete(fitment: EquipmentFitment) {
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
            setNotice("Maquinaria eliminada.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar la maquinaria."))
        }
    }

    function filterSelect(
        value: string,
        onChange: (value: string) => void,
        items: { label: string; value: string }[],
        label: string,
        loading: boolean,
    ) {
        return (
            <Select
                value={value}
                onValueChange={(next) => {
                    onChange(next ?? ALL)
                    setOffset(0)
                }}
                items={items}
                disabled={loading}
            >
                <SelectTrigger className="w-48" aria-label={label}>
                    <SelectValue />
                </SelectTrigger>
                <SelectContent align="start">
                    <SelectGroup>
                        {items.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                                {item.label}
                            </SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
        )
    }

    const hasFilters = debouncedQuery.trim() !== "" || brandId !== ALL || typeId !== ALL

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Maquinaria</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Catálogo de maquinaria (marca, tipo, modelo y/o serie) a la que se vinculan las compatibilidades de los productos.
                    </p>
                </div>
                <Button size="sm" onClick={() => openEditor(null)}>
                    <PlusIcon />
                    Nueva maquinaria
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
                        placeholder="Buscar por marca, tipo, modelo o serie"
                        aria-label="Buscar maquinaria"
                        className="pl-8"
                    />
                </div>
                {filterSelect(brandId, setBrandId, brandItems, "Filtrar por marca", brands.isLoading)}
                {filterSelect(typeId, setTypeId, typeItems, "Filtrar por tipo", types.isLoading)}
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener la maquinaria: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Marca</TableHead>
                        <TableHead>Tipo</TableHead>
                        <TableHead>Modelo</TableHead>
                        <TableHead>Serie</TableHead>
                        <TableHead>Productos</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && fitments.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                Cargando maquinaria...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && fitments.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                {hasFilters
                                    ? "Ninguna maquinaria coincide con la búsqueda."
                                    : "Aún no hay maquinaria. Crea la primera con «Nueva maquinaria»."}
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {fitments.map((fitment) => (
                        <TableRow key={fitment.id}>
                            <TableCell>{fitment.brandName || "—"}</TableCell>
                            <TableCell>{fitment.equipmentTypeName || "—"}</TableCell>
                            <TableCell className={fitment.model ? "" : "text-muted-foreground"}>{fitment.model || "—"}</TableCell>
                            <TableCell className={fitment.serie ? "" : "text-muted-foreground"}>{fitment.serie || "—"}</TableCell>
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
                                        aria-label={`Editar ${fitmentLabel(fitment)}`}
                                        onClick={() => openEditor(fitment)}
                                    >
                                        <PencilIcon />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Eliminar ${fitmentLabel(fitment)}`}
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

            <EquipmentFitmentDialog open={editorOpen} onOpenChange={setEditorOpen} item={editing} onSaved={setNotice} />

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar maquinaria"
                description={
                    deleting ? (
                        <>
                            Se eliminará <strong>{fitmentLabel(deleting)}</strong>.
                            {deleting.productCount > 0
                                ? ` Está vinculada a ${formatProductCount(deleting.productCount)}: también se les quitará esta compatibilidad.`
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

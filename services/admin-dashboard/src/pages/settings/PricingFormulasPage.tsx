import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { TablePagination } from "@/components/TablePagination"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import { useBrandOptions, useChannelConnections } from "@/lib/product-detail"
import { paginatedPriceListsResponseSchema } from "@/lib/schemas/price-lists"
import {
    paginatedPricingFormulasResponseSchema,
    pricingFormulaSchema,
    type PricingFormula,
} from "@/lib/schemas/pricing-formulas"
import { usePricingFormulasPaginationStore } from "@/stores/pricingFormulasPaginationStore"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

// Valor centinela del selector para "cualquiera" (el backend lo recibe como null).
const ANY = "any"

type SaveFormulaInput = {
    id?: number
    brandId: number | null
    connectionId: number | null
    priceListId: number | null
    expression: string
    description: string | null
}

type Lookups = {
    brandName: (id: number | null) => string
    connectionName: (id: number | null) => string
    priceListName: (id: number | null) => string
    brandItems: { label: string; value: string }[]
    connectionItems: { label: string; value: string }[]
    priceListItems: { label: string; value: string }[]
}

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

function toId(value: string): number | null {
    return value === ANY ? null : Number(value)
}

/** Catálogos para mostrar nombres y poblar los selectores de alcance. */
function useLookups(): Lookups {
    const { data: brands } = useBrandOptions()
    const { data: connections } = useChannelConnections()
    const { data: priceLists } = useQuery({
        queryKey: ["price-lists", "lookup"],
        staleTime: 5 * 60 * 1000,
        queryFn: async () => {
            const response = await apiClient.get("/api/price-lists", { params: { offset: 0, pageSize: 100 } })
            return paginatedPriceListsResponseSchema.parse(response.data).data
        },
    })

    const brandById = new Map((brands ?? []).map((b) => [b.id, b.name]))
    const connectionById = new Map((connections ?? []).map((c) => [c.id, c.name]))
    const priceListById = new Map((priceLists ?? []).map((p) => [p.id, p.name]))

    const nameOf = (map: Map<number, string>) => (id: number | null) =>
        id === null ? "Todas" : (map.get(id) ?? `#${id}`)

    return {
        brandName: nameOf(brandById),
        connectionName: nameOf(connectionById),
        priceListName: nameOf(priceListById),
        brandItems: [{ label: "Todas las marcas", value: ANY }, ...(brands ?? []).map((b) => ({ label: b.name, value: String(b.id) }))],
        connectionItems: [
            { label: "Todas las conexiones", value: ANY },
            ...(connections ?? []).map((c) => ({ label: c.name, value: String(c.id) })),
        ],
        priceListItems: [
            { label: "Todas las listas", value: ANY },
            ...(priceLists ?? []).map((p) => ({ label: p.name, value: String(p.id) })),
        ],
    }
}

function useInvalidateFormulas() {
    const queryClient = useQueryClient()
    return () => queryClient.invalidateQueries({ queryKey: ["pricing-formulas"] })
}

/** Alta o edición de una fórmula (POST / PUT /api/pricing-formulas). */
function useSaveFormula() {
    const invalidate = useInvalidateFormulas()
    return useMutation({
        mutationFn: async (input: SaveFormulaInput) => {
            const response = input.id
                ? await apiClient.put(`/api/pricing-formulas/${input.id}`, input)
                : await apiClient.post("/api/pricing-formulas", input)
            return pricingFormulaSchema.parse(response.data)
        },
        onSuccess: invalidate,
    })
}

function useDeleteFormula() {
    const invalidate = useInvalidateFormulas()
    return useMutation({
        mutationFn: async (id: number) => {
            await apiClient.delete(`/api/pricing-formulas/${id}`)
        },
        onSuccess: invalidate,
    })
}

function ScopeSelect({
    id,
    label,
    value,
    onChange,
    items,
    disabled,
}: {
    id: string
    label: string
    value: string
    onChange: (value: string) => void
    items: { label: string; value: string }[]
    disabled: boolean
}) {
    return (
        <div className="flex flex-col gap-1.5">
            <Label htmlFor={id}>{label}</Label>
            <Select items={items} value={value} onValueChange={(next) => onChange(next ?? ANY)} disabled={disabled}>
                <SelectTrigger id={id} className="w-full">
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
        </div>
    )
}

function FormulaForm({
    item,
    lookups,
    onClose,
    onSaved,
}: {
    item: PricingFormula | null
    lookups: Lookups
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const [brand, setBrand] = useState(item?.brandId != null ? String(item.brandId) : ANY)
    const [connection, setConnection] = useState(item?.connectionId != null ? String(item.connectionId) : ANY)
    const [priceList, setPriceList] = useState(item?.priceListId != null ? String(item.priceListId) : ANY)
    const [expression, setExpression] = useState(item?.expression ?? "")
    const [description, setDescription] = useState(item?.description ?? "")
    const [error, setError] = useState<string | null>(null)
    const mutation = useSaveFormula()

    // El alcance define la "ranura" de la fórmula y el backend no permite moverla: solo se elige al crear.
    const scopeLocked = item !== null

    async function submit() {
        const trimmed = expression.trim()
        if (!trimmed) {
            setError("La expresión es obligatoria.")
            return
        }
        setError(null)

        try {
            await mutation.mutateAsync({
                id: item?.id,
                brandId: toId(brand),
                connectionId: toId(connection),
                priceListId: toId(priceList),
                expression: trimmed,
                description: description.trim() || null,
            })
            onSaved(item ? "Fórmula actualizada." : "Fórmula creada.")
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar la fórmula."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{item ? "Editar fórmula" : "Nueva fórmula"}</DialogTitle>
                <DialogDescription>
                    {item
                        ? "El alcance (marca, conexión y lista) no se puede cambiar; para otro alcance crea una fórmula nueva."
                        : "Deja un campo en \"Todas\" para que la fórmula aplique a cualquier valor. Solo puede haber una fórmula por combinación."}
                </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-3">
                <ScopeSelect
                    id="formula-brand"
                    label="Marca"
                    value={brand}
                    onChange={setBrand}
                    items={lookups.brandItems}
                    disabled={scopeLocked || mutation.isPending}
                />
                <ScopeSelect
                    id="formula-connection"
                    label="Conexión"
                    value={connection}
                    onChange={setConnection}
                    items={lookups.connectionItems}
                    disabled={scopeLocked || mutation.isPending}
                />
                <ScopeSelect
                    id="formula-price-list"
                    label="Lista de precios"
                    value={priceList}
                    onChange={setPriceList}
                    items={lookups.priceListItems}
                    disabled={scopeLocked || mutation.isPending}
                />

                <div className="flex flex-col gap-1.5">
                    <Label htmlFor="formula-expression">Expresión</Label>
                    <Input
                        id="formula-expression"
                        value={expression}
                        onChange={(event) => setExpression(event.target.value)}
                        disabled={mutation.isPending}
                        aria-invalid={error ? true : undefined}
                        placeholder="base * 1.16 * 1.25"
                        className="font-mono"
                        autoFocus
                    />
                    <p className="text-xs text-muted-foreground">
                        La única variable es <code className="font-mono">base</code> (precio base en MXN). El resultado se
                        redondea a 2 decimales. Ej.: <code className="font-mono">base * 1.3 + 50</code>
                    </p>
                </div>

                <div className="flex flex-col gap-1.5">
                    <Label htmlFor="formula-description">Descripción (opcional)</Label>
                    <Textarea
                        id="formula-description"
                        value={description}
                        onChange={(event) => setDescription(event.target.value)}
                        disabled={mutation.isPending}
                        rows={2}
                    />
                </div>

                {error ? (
                    <p className="text-xs text-destructive" role="alert">
                        {error}
                    </p>
                ) : null}
            </div>

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={mutation.isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={() => void submit()} disabled={mutation.isPending}>
                    {mutation.isPending ? "Guardando…" : item ? "Guardar" : "Crear"}
                </Button>
            </DialogFooter>
        </>
    )
}

export function PricingFormulasPage() {
    const offset = usePricingFormulasPaginationStore((state) => state.offset)
    const pageSize = usePricingFormulasPaginationStore((state) => state.pageSize)
    const setOffset = usePricingFormulasPaginationStore((state) => state.setOffset)
    const setPageSize = usePricingFormulasPaginationStore((state) => state.setPageSize)

    const [notice, setNotice] = useState<string | null>(null)
    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<PricingFormula | null>(null)
    const [deleting, setDeleting] = useState<PricingFormula | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)
    const deleteMutation = useDeleteFormula()
    const lookups = useLookups()

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["pricing-formulas", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/pricing-formulas", { params: { offset, pageSize } })
            return paginatedPricingFormulasResponseSchema.parse(response.data)
        },
    })

    const formulas = data?.data ?? []
    const total = data?.total ?? 0

    function openEditor(formula: PricingFormula | null) {
        setEditing(formula)
        setEditorOpen(true)
    }

    async function confirmDelete() {
        if (!deleting) {
            return
        }
        setDeleteError(null)
        try {
            await deleteMutation.mutateAsync(deleting.id)
            setNotice("Fórmula eliminada.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar la fórmula."))
        }
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Fórmulas de precios</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Convierten el precio base MXN en el precio que se publica en cada marketplace. Se aplica la más
                        específica: primero la que coincide con la conexión, luego marca y luego lista de precios; la fila
                        con todo en &quot;Todas&quot; es el valor por defecto.
                    </p>
                </div>
                <Button size="sm" onClick={() => openEditor(null)}>
                    <PlusIcon />
                    Nueva fórmula
                </Button>
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las fórmulas: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Marca</TableHead>
                        <TableHead>Conexión</TableHead>
                        <TableHead>Lista de precios</TableHead>
                        <TableHead>Expresión</TableHead>
                        <TableHead>Descripción</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && formulas.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                Cargando fórmulas...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && formulas.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                No se encontraron fórmulas de precios.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {formulas.map((formula) => (
                        <TableRow key={formula.id}>
                            <TableCell className={formula.brandId === null ? "text-muted-foreground" : ""}>
                                {lookups.brandName(formula.brandId)}
                            </TableCell>
                            <TableCell className={formula.connectionId === null ? "text-muted-foreground" : ""}>
                                {lookups.connectionName(formula.connectionId)}
                            </TableCell>
                            <TableCell className={formula.priceListId === null ? "text-muted-foreground" : ""}>
                                {lookups.priceListName(formula.priceListId)}
                            </TableCell>
                            <TableCell className="font-mono text-xs">{formula.expression}</TableCell>
                            <TableCell className="max-w-64 truncate text-muted-foreground" title={formula.description ?? undefined}>
                                {formula.description ?? "-"}
                            </TableCell>
                            <TableCell>{formatDate(formula.updatedAt)}</TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Editar fórmula ${formula.id}`}
                                        onClick={() => openEditor(formula)}
                                    >
                                        <PencilIcon />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Eliminar fórmula ${formula.id}`}
                                        onClick={() => {
                                            setDeleteError(null)
                                            setDeleting(formula)
                                        }}
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
                onPageSizeChange={setPageSize}
            />

            <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
                <DialogContent>
                    <FormulaForm
                        item={editing}
                        lookups={lookups}
                        onClose={() => setEditorOpen(false)}
                        onSaved={setNotice}
                    />
                </DialogContent>
            </Dialog>

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar fórmula"
                description={
                    deleting ? (
                        <>
                            Se eliminará la fórmula <strong className="font-mono">{deleting.expression}</strong>. Los
                            productos de su alcance usarán la siguiente fórmula más general.
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

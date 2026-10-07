import { EquipmentFitmentFields, VehicleFitmentFields } from "@/components/fitments/FitmentFields"
import { Badge } from "@/components/product-detail/ProductDetailPrimitives"
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
import { useDebouncedValue } from "@/hooks/use-debounced-value"
import { getServerErrorMessage } from "@/lib/api"
import {
    emptyEquipmentDraft,
    emptyVehicleDraft,
    formatEquipmentModel,
    formatProductCount,
    formatYearRange,
    useEquipmentFitments,
    useFindOrCreateEquipmentFitment,
    useFindOrCreateVehicleFitment,
    useLinkEquipmentCompatibility,
    useLinkVehicleCompatibility,
    useVehicleFitments,
    validateEquipmentDraft,
    validateVehicleDraft,
} from "@/lib/fitments"
import type { EquipmentFitment, VehicleFitment } from "@/lib/schemas/fitments"
import { cn } from "@/lib/utils"
import axios from "axios"
import { SearchIcon } from "lucide-react"
import { useState } from "react"

const SEARCH_PAGE_SIZE = 8

type Kind = "vehicle" | "equipment"
type Mode = "search" | "create"

/**
 * Agrega una compatibilidad al producto: se busca y elige un vehículo o una
 * maquinaria que ya exista, o se crea uno nuevo desde la misma ventana. Crear
 * nunca duplica: si ya hay un registro con la misma combinación, el backend
 * devuelve el existente y es el que se vincula.
 */
export function AddCompatibilityDialog({
    productId,
    open,
    onOpenChange,
    linkedVehicleFitmentIds,
    linkedEquipmentFitmentIds,
    onAdded,
}: {
    productId: string | undefined
    open: boolean
    onOpenChange: (open: boolean) => void
    linkedVehicleFitmentIds: number[]
    linkedEquipmentFitmentIds: number[]
    /** Mensaje para mostrar en la sección al terminar. */
    onAdded: (message: string) => void
}) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-xl">
                <AddCompatibilityBody
                    productId={productId}
                    linkedVehicleFitmentIds={linkedVehicleFitmentIds}
                    linkedEquipmentFitmentIds={linkedEquipmentFitmentIds}
                    onClose={() => onOpenChange(false)}
                    onAdded={onAdded}
                />
            </DialogContent>
        </Dialog>
    )
}

function AddCompatibilityBody({
    productId,
    linkedVehicleFitmentIds,
    linkedEquipmentFitmentIds,
    onClose,
    onAdded,
}: {
    productId: string | undefined
    linkedVehicleFitmentIds: number[]
    linkedEquipmentFitmentIds: number[]
    onClose: () => void
    onAdded: (message: string) => void
}) {
    const [kind, setKind] = useState<Kind>("vehicle")

    function finish(message: string) {
        onAdded(message)
        onClose()
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>Agregar compatibilidad</DialogTitle>
                <DialogDescription>
                    Busca el vehículo o la maquinaria a los que aplica este producto. Si no existe, puedes crearlo aquí mismo.
                </DialogDescription>
            </DialogHeader>

            <div className="flex gap-1.5" role="group" aria-label="Tipo de compatibilidad">
                <SegmentButton active={kind === "vehicle"} onClick={() => setKind("vehicle")}>
                    Vehículo
                </SegmentButton>
                <SegmentButton active={kind === "equipment"} onClick={() => setKind("equipment")}>
                    Maquinaria
                </SegmentButton>
            </div>

            {kind === "vehicle" ? (
                <VehiclePanel
                    productId={productId}
                    linkedIds={linkedVehicleFitmentIds}
                    onClose={onClose}
                    onDone={finish}
                />
            ) : (
                <EquipmentPanel
                    productId={productId}
                    linkedIds={linkedEquipmentFitmentIds}
                    onClose={onClose}
                    onDone={finish}
                />
            )}
        </>
    )
}

function SegmentButton({
    active,
    onClick,
    children,
}: {
    active: boolean
    onClick: () => void
    children: React.ReactNode
}) {
    return (
        <Button type="button" size="sm" variant={active ? "default" : "outline"} aria-pressed={active} onClick={onClick}>
            {children}
        </Button>
    )
}

/** Mensaje legible para un fallo al vincular; el 409 es "ya estaba vinculada". */
function linkErrorMessage(error: unknown): string {
    if (axios.isAxiosError(error) && error.response?.status === 409) {
        return "Este producto ya tiene esa compatibilidad."
    }
    return getServerErrorMessage(error, "No se pudo agregar la compatibilidad.")
}

function ModeSwitch({ mode, onChange, disabled }: { mode: Mode; onChange: (mode: Mode) => void; disabled: boolean }) {
    return (
        <div className="flex gap-1.5" role="group" aria-label="Modo">
            <SegmentButton active={mode === "search"} onClick={() => !disabled && onChange("search")}>
                Buscar existente
            </SegmentButton>
            <SegmentButton active={mode === "create"} onClick={() => !disabled && onChange("create")}>
                Crear nuevo
            </SegmentButton>
        </div>
    )
}

function SearchBox({
    value,
    onChange,
    placeholder,
}: {
    value: string
    onChange: (value: string) => void
    placeholder: string
}) {
    return (
        <div className="relative">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
                value={value}
                onChange={(event) => onChange(event.target.value)}
                placeholder={placeholder}
                aria-label={placeholder}
                className="pl-8"
                autoFocus
            />
        </div>
    )
}

function ResultList<T extends { id: number; productCount: number }>({
    items,
    isLoading,
    isError,
    selectedId,
    linkedIds,
    hasQuery,
    onSelect,
    onCreate,
    renderPrimary,
    renderSecondary,
}: {
    items: T[]
    isLoading: boolean
    isError: boolean
    selectedId: number | null
    linkedIds: number[]
    hasQuery: boolean
    onSelect: (item: T) => void
    onCreate: () => void
    renderPrimary: (item: T) => string
    renderSecondary: (item: T) => string
}) {
    if (isLoading && items.length === 0) {
        return <p className="py-6 text-center text-sm text-muted-foreground">Buscando…</p>
    }
    if (isError) {
        return <p className="py-6 text-center text-sm text-destructive">No se pudo buscar en el catálogo.</p>
    }
    if (items.length === 0) {
        return (
            <div className="flex flex-col items-center gap-2 py-6 text-center">
                <p className="text-sm text-muted-foreground">
                    {hasQuery ? "No hay resultados para esa búsqueda." : "El catálogo está vacío."}
                </p>
                <Button type="button" variant="outline" size="sm" onClick={onCreate}>
                    Crear nuevo
                </Button>
            </div>
        )
    }

    return (
        <ul className="max-h-60 divide-y divide-border overflow-y-auto rounded-md border border-border">
            {items.map((item) => {
                const selected = item.id === selectedId
                return (
                    <li key={item.id}>
                        <button
                            type="button"
                            aria-pressed={selected}
                            onClick={() => onSelect(item)}
                            className={cn(
                                "flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm outline-none transition-colors hover:bg-muted focus-visible:bg-muted",
                                selected && "bg-muted",
                            )}
                        >
                            <span className="min-w-0">
                                <span className="block font-medium wrap-break-word text-foreground">{renderPrimary(item)}</span>
                                <span className="block text-xs text-muted-foreground">{renderSecondary(item)}</span>
                            </span>
                            <span className="flex shrink-0 items-center gap-1.5">
                                {linkedIds.includes(item.id) ? <Badge tone="warning">Ya vinculado</Badge> : null}
                                <Badge tone="muted">{formatProductCount(item.productCount)}</Badge>
                            </span>
                        </button>
                    </li>
                )
            })}
        </ul>
    )
}

function ErrorLine({ message }: { message: string | null }) {
    if (!message) {
        return null
    }
    return (
        <p className="text-xs text-destructive" role="alert">
            {message}
        </p>
    )
}

function VehiclePanel({
    productId,
    linkedIds,
    onClose,
    onDone,
}: {
    productId: string | undefined
    linkedIds: number[]
    onClose: () => void
    onDone: (message: string) => void
}) {
    const [mode, setMode] = useState<Mode>("search")
    const [query, setQuery] = useState("")
    const [selected, setSelected] = useState<VehicleFitment | null>(null)
    const [draft, setDraft] = useState(() => emptyVehicleDraft())
    const [motor, setMotor] = useState("")
    const [position, setPosition] = useState("")
    const [side, setSide] = useState("")
    const [error, setError] = useState<string | null>(null)

    const debouncedQuery = useDebouncedValue(query)
    const search = useVehicleFitments(
        { q: debouncedQuery, brandId: null, offset: 0, pageSize: SEARCH_PAGE_SIZE },
        mode === "search",
    )
    const findOrCreate = useFindOrCreateVehicleFitment()
    const link = useLinkVehicleCompatibility(productId)
    const isPending = findOrCreate.isPending || link.isPending
    const canSubmit = mode === "create" || selected !== null

    async function submit() {
        setError(null)

        let fitmentId: number
        let message = "Compatibilidad agregada."

        try {
            if (mode === "search") {
                if (!selected) {
                    setError("Elige un vehículo de la lista.")
                    return
                }
                fitmentId = selected.id
            } else {
                const result = validateVehicleDraft(draft)
                if (!result.ok) {
                    setError(result.message)
                    return
                }
                const resolved = await findOrCreate.mutateAsync(result.payload)
                fitmentId = resolved.fitment.id
                message = resolved.created
                    ? "Se creó el vehículo y se agregó la compatibilidad."
                    : "Ya existía ese vehículo: se usó el existente y se agregó la compatibilidad."
            }

            await link.mutateAsync({
                vehicleFitmentId: fitmentId,
                motor: motor.trim(),
                position: position.trim(),
                side: side.trim(),
            })
            onDone(message)
        } catch (submitError) {
            setError(linkErrorMessage(submitError))
        }
    }

    return (
        <>
            <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
                <ModeSwitch mode={mode} onChange={setMode} disabled={isPending} />

                {mode === "search" ? (
                    <>
                        <SearchBox value={query} onChange={setQuery} placeholder="Buscar por marca, modelo o año" />
                        <ResultList
                            items={search.data?.fitments ?? []}
                            isLoading={search.isLoading}
                            isError={search.isError}
                            selectedId={selected?.id ?? null}
                            linkedIds={linkedIds}
                            hasQuery={debouncedQuery.trim() !== ""}
                            onSelect={setSelected}
                            onCreate={() => setMode("create")}
                            renderPrimary={(item) => `${item.brandName} ${item.model}`}
                            renderSecondary={(item) => formatYearRange(item.yearStart, item.yearEnd)}
                        />
                        {search.data && search.data.total > SEARCH_PAGE_SIZE ? (
                            <p className="text-xs text-muted-foreground">
                                Mostrando {SEARCH_PAGE_SIZE} de {search.data.total}. Afina la búsqueda para ver otros.
                            </p>
                        ) : null}
                    </>
                ) : (
                    <VehicleFitmentFields draft={draft} onChange={setDraft} disabled={isPending} />
                )}

                {canSubmit ? (
                    <fieldset className="flex flex-col gap-2 border-t border-border pt-3" disabled={isPending}>
                        <legend className="sr-only">Calificadores</legend>
                        <p className="text-xs text-muted-foreground">
                            Calificadores opcionales: cómo se monta este producto en el vehículo (los usa MercadoLibre).
                        </p>
                        <div className="grid grid-cols-3 gap-3">
                            <QualifierInput label="Motor" value={motor} onChange={setMotor} placeholder="2.0L" />
                            <QualifierInput label="Posición" value={position} onChange={setPosition} placeholder="Delantero" />
                            <QualifierInput label="Lado" value={side} onChange={setSide} placeholder="Izquierdo" />
                        </div>
                    </fieldset>
                ) : null}
            </div>

            <ErrorLine message={error} />
            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={submit} disabled={isPending || !canSubmit}>
                    {isPending ? "Agregando…" : "Agregar compatibilidad"}
                </Button>
            </DialogFooter>
        </>
    )
}

function QualifierInput({
    label,
    value,
    onChange,
    placeholder,
}: {
    label: string
    value: string
    onChange: (value: string) => void
    placeholder: string
}) {
    return (
        <div className="flex flex-col gap-1.5">
            <Label>{label}</Label>
            <Input
                value={value}
                onChange={(event) => onChange(event.target.value)}
                placeholder={placeholder}
                maxLength={100}
                aria-label={label}
            />
        </div>
    )
}

function EquipmentPanel({
    productId,
    linkedIds,
    onClose,
    onDone,
}: {
    productId: string | undefined
    linkedIds: number[]
    onClose: () => void
    onDone: (message: string) => void
}) {
    const [mode, setMode] = useState<Mode>("search")
    const [query, setQuery] = useState("")
    const [selected, setSelected] = useState<EquipmentFitment | null>(null)
    const [draft, setDraft] = useState(() => emptyEquipmentDraft())
    const [error, setError] = useState<string | null>(null)

    const debouncedQuery = useDebouncedValue(query)
    const search = useEquipmentFitments(
        { q: debouncedQuery, brandId: null, equipmentTypeId: null, offset: 0, pageSize: SEARCH_PAGE_SIZE },
        mode === "search",
    )
    const findOrCreate = useFindOrCreateEquipmentFitment()
    const link = useLinkEquipmentCompatibility(productId)
    const isPending = findOrCreate.isPending || link.isPending
    const canSubmit = mode === "create" || selected !== null

    async function submit() {
        setError(null)

        let fitmentId: number
        let message = "Compatibilidad agregada."

        try {
            if (mode === "search") {
                if (!selected) {
                    setError("Elige una maquinaria de la lista.")
                    return
                }
                fitmentId = selected.id
            } else {
                const result = validateEquipmentDraft(draft)
                if (!result.ok) {
                    setError(result.message)
                    return
                }
                const resolved = await findOrCreate.mutateAsync(result.payload)
                fitmentId = resolved.fitment.id
                message = resolved.created
                    ? "Se creó la maquinaria y se agregó la compatibilidad."
                    : "Ya existía esa maquinaria: se usó la existente y se agregó la compatibilidad."
            }

            await link.mutateAsync(fitmentId)
            onDone(message)
        } catch (submitError) {
            setError(linkErrorMessage(submitError))
        }
    }

    return (
        <>
            <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
                <ModeSwitch mode={mode} onChange={setMode} disabled={isPending} />

                {mode === "search" ? (
                    <>
                        <SearchBox value={query} onChange={setQuery} placeholder="Buscar por marca, tipo, modelo o serie" />
                        <ResultList
                            items={search.data?.fitments ?? []}
                            isLoading={search.isLoading}
                            isError={search.isError}
                            selectedId={selected?.id ?? null}
                            linkedIds={linkedIds}
                            hasQuery={debouncedQuery.trim() !== ""}
                            onSelect={setSelected}
                            onCreate={() => setMode("create")}
                            renderPrimary={(item) => `${item.brandName} · ${item.equipmentTypeName}`}
                            renderSecondary={(item) => formatEquipmentModel(item.model, item.serie)}
                        />
                        {search.data && search.data.total > SEARCH_PAGE_SIZE ? (
                            <p className="text-xs text-muted-foreground">
                                Mostrando {SEARCH_PAGE_SIZE} de {search.data.total}. Afina la búsqueda para ver otras.
                            </p>
                        ) : null}
                    </>
                ) : (
                    <EquipmentFitmentFields draft={draft} onChange={setDraft} disabled={isPending} />
                )}
            </div>

            <ErrorLine message={error} />
            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={submit} disabled={isPending || !canSubmit}>
                    {isPending ? "Agregando…" : "Agregar compatibilidad"}
                </Button>
            </DialogFooter>
        </>
    )
}

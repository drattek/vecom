import { EquipmentFitmentFields, VehicleFitmentFields } from "@/components/fitments/FitmentFields"
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
import { getServerErrorMessage } from "@/lib/api"
import {
    emptyEquipmentDraft,
    emptyVehicleDraft,
    equipmentDraftFromFitment,
    useFindOrCreateEquipmentFitment,
    useFindOrCreateVehicleFitment,
    useSaveEquipmentType,
    useUpdateEquipmentFitment,
    useUpdateVehicleFitment,
    validateEquipmentDraft,
    validateVehicleDraft,
    vehicleDraftFromFitment,
} from "@/lib/fitments"
import type { EquipmentFitment, EquipmentType, VehicleFitment } from "@/lib/schemas/fitments"
import { useState } from "react"

// Diálogos de alta / edición de Settings → Vehículos/Maquinaria. El formulario
// vive en un hijo de DialogContent: el popup se desmonta al cerrar, así que el
// borrador se reinicia solo en cada apertura.

const ALREADY_EXISTS_NOTICE = "Ya existía un registro con esos datos: no se creó uno nuevo."

type DialogProps<T> = {
    open: boolean
    onOpenChange: (open: boolean) => void
    /** Fila a editar; `null` para crear. */
    item: T | null
    /** Mensaje para mostrar en la página al terminar (p. ej. "ya existía"). */
    onSaved: (message: string) => void
}

function FormError({ message }: { message: string | null }) {
    if (!message) {
        return null
    }
    return (
        <p className="text-xs text-destructive" role="alert">
            {message}
        </p>
    )
}

export function VehicleFitmentDialog({ open, onOpenChange, item, onSaved }: DialogProps<VehicleFitment>) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <VehicleFitmentForm item={item} onClose={() => onOpenChange(false)} onSaved={onSaved} />
            </DialogContent>
        </Dialog>
    )
}

function VehicleFitmentForm({
    item,
    onClose,
    onSaved,
}: {
    item: VehicleFitment | null
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const [draft, setDraft] = useState(() => (item ? vehicleDraftFromFitment(item) : emptyVehicleDraft()))
    const [error, setError] = useState<string | null>(null)
    const createMutation = useFindOrCreateVehicleFitment()
    const updateMutation = useUpdateVehicleFitment()
    const isPending = createMutation.isPending || updateMutation.isPending

    async function submit() {
        const result = validateVehicleDraft(draft)
        if (!result.ok) {
            setError(result.message)
            return
        }
        setError(null)

        try {
            if (item) {
                if (!result.payload.brandId) {
                    setError("Elige una marca existente.")
                    return
                }
                await updateMutation.mutateAsync({
                    id: item.id,
                    brandId: result.payload.brandId,
                    model: result.payload.model,
                    yearStart: result.payload.yearStart,
                    yearEnd: result.payload.yearEnd,
                })
                onSaved("Vehículo actualizado.")
            } else {
                const created = await createMutation.mutateAsync(result.payload)
                onSaved(created.created ? "Vehículo creado." : ALREADY_EXISTS_NOTICE)
            }
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar el vehículo."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{item ? "Editar vehículo" : "Nuevo vehículo"}</DialogTitle>
                <DialogDescription>
                    {item
                        ? "Cambiar estos datos afecta a todos los productos vinculados a este vehículo."
                        : "Si ya existe un vehículo con la misma marca, modelo y años, se reutiliza en lugar de duplicarlo."}
                </DialogDescription>
            </DialogHeader>

            <div className="min-h-0 overflow-y-auto">
                <VehicleFitmentFields draft={draft} onChange={setDraft} disabled={isPending} allowNewBrand={!item} />
            </div>
            <FormError message={error} />

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={submit} disabled={isPending}>
                    {isPending ? "Guardando…" : item ? "Guardar" : "Crear"}
                </Button>
            </DialogFooter>
        </>
    )
}

export function EquipmentFitmentDialog({ open, onOpenChange, item, onSaved }: DialogProps<EquipmentFitment>) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <EquipmentFitmentForm item={item} onClose={() => onOpenChange(false)} onSaved={onSaved} />
            </DialogContent>
        </Dialog>
    )
}

function EquipmentFitmentForm({
    item,
    onClose,
    onSaved,
}: {
    item: EquipmentFitment | null
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const [draft, setDraft] = useState(() => (item ? equipmentDraftFromFitment(item) : emptyEquipmentDraft()))
    const [error, setError] = useState<string | null>(null)
    const createMutation = useFindOrCreateEquipmentFitment()
    const updateMutation = useUpdateEquipmentFitment()
    const isPending = createMutation.isPending || updateMutation.isPending

    async function submit() {
        const result = validateEquipmentDraft(draft)
        if (!result.ok) {
            setError(result.message)
            return
        }
        setError(null)

        try {
            if (item) {
                if (!result.payload.brandId || !result.payload.equipmentTypeId) {
                    setError("Elige una marca y un tipo existentes.")
                    return
                }
                await updateMutation.mutateAsync({
                    id: item.id,
                    brandId: result.payload.brandId,
                    equipmentTypeId: result.payload.equipmentTypeId,
                    model: result.payload.model,
                    serie: result.payload.serie,
                })
                onSaved("Maquinaria actualizada.")
            } else {
                const created = await createMutation.mutateAsync(result.payload)
                onSaved(created.created ? "Maquinaria creada." : ALREADY_EXISTS_NOTICE)
            }
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar la maquinaria."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{item ? "Editar maquinaria" : "Nueva maquinaria"}</DialogTitle>
                <DialogDescription>
                    {item
                        ? "Cambiar estos datos afecta a todos los productos vinculados a esta maquinaria."
                        : "Si ya existe una maquinaria con la misma marca, tipo, modelo y serie, se reutiliza en lugar de duplicarla."}
                </DialogDescription>
            </DialogHeader>

            <div className="min-h-0 overflow-y-auto">
                <EquipmentFitmentFields draft={draft} onChange={setDraft} disabled={isPending} allowNew={!item} />
            </div>
            <FormError message={error} />

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={submit} disabled={isPending}>
                    {isPending ? "Guardando…" : item ? "Guardar" : "Crear"}
                </Button>
            </DialogFooter>
        </>
    )
}

export function EquipmentTypeDialog({ open, onOpenChange, item, onSaved }: DialogProps<EquipmentType>) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <EquipmentTypeForm item={item} onClose={() => onOpenChange(false)} onSaved={onSaved} />
            </DialogContent>
        </Dialog>
    )
}

function EquipmentTypeForm({
    item,
    onClose,
    onSaved,
}: {
    item: EquipmentType | null
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const [name, setName] = useState(item?.name ?? "")
    const [error, setError] = useState<string | null>(null)
    const mutation = useSaveEquipmentType()

    async function submit() {
        const trimmed = name.trim()
        if (!trimmed) {
            setError("El nombre es obligatorio.")
            return
        }
        setError(null)

        try {
            await mutation.mutateAsync({ id: item?.id, name: trimmed })
            onSaved(item ? "Tipo actualizado." : "Tipo creado.")
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar el tipo."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{item ? "Editar tipo de maquinaria" : "Nuevo tipo de maquinaria"}</DialogTitle>
                <DialogDescription>
                    Clasificación de la maquinaria (montacargas, plataformas, etc.). Los nombres no se repiten.
                </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-1.5">
                <Label htmlFor="equipment-type-name">Nombre</Label>
                <Input
                    id="equipment-type-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    disabled={mutation.isPending}
                    aria-invalid={error ? true : undefined}
                    maxLength={255}
                    autoFocus
                    onKeyDown={(event) => {
                        if (event.key === "Enter") {
                            event.preventDefault()
                            void submit()
                        }
                    }}
                />
            </div>
            <FormError message={error} />

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={mutation.isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                    {mutation.isPending ? "Guardando…" : item ? "Guardar" : "Crear"}
                </Button>
            </DialogFooter>
        </>
    )
}

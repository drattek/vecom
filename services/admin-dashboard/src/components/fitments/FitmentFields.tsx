import { CatalogSelect } from "@/components/fitments/CatalogSelect"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    MAX_FITMENT_YEAR,
    MIN_FITMENT_YEAR,
    useEquipmentTypes,
    type EquipmentFitmentDraft,
    type VehicleFitmentDraft,
} from "@/lib/fitments"
import { useBrandOptions } from "@/lib/product-detail"
import { useId } from "react"

/** Campos de un fitment de vehículo: marca, modelo y rango de años. */
export function VehicleFitmentFields({
    draft,
    onChange,
    disabled,
    allowNewBrand = true,
}: {
    draft: VehicleFitmentDraft
    onChange: (draft: VehicleFitmentDraft) => void
    disabled?: boolean
    allowNewBrand?: boolean
}) {
    const brands = useBrandOptions()
    const ids = useId()

    return (
        <div className="flex flex-col gap-4">
            <CatalogSelect
                label="Marca"
                items={brands.data ?? []}
                value={draft.brand}
                onChange={(brand) => onChange({ ...draft, brand })}
                placeholder="— Selecciona una marca —"
                newLabel="Marca nueva"
                allowCreate={allowNewBrand}
                loading={brands.isLoading}
                disabled={disabled}
            />

            <div className="flex flex-col gap-1.5">
                <Label htmlFor={`${ids}-model`}>Modelo</Label>
                <Input
                    id={`${ids}-model`}
                    value={draft.model}
                    onChange={(event) => onChange({ ...draft, model: event.target.value })}
                    placeholder="Ej. Corolla"
                    disabled={disabled}
                    maxLength={255}
                />
            </div>

            <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor={`${ids}-from`}>Año inicial</Label>
                    <Input
                        id={`${ids}-from`}
                        type="number"
                        inputMode="numeric"
                        min={MIN_FITMENT_YEAR}
                        max={MAX_FITMENT_YEAR}
                        value={draft.yearStart}
                        onChange={(event) => onChange({ ...draft, yearStart: event.target.value })}
                        placeholder="2015"
                        disabled={disabled}
                    />
                </div>
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor={`${ids}-to`}>Año final (opcional)</Label>
                    <Input
                        id={`${ids}-to`}
                        type="number"
                        inputMode="numeric"
                        min={MIN_FITMENT_YEAR}
                        max={MAX_FITMENT_YEAR}
                        value={draft.yearEnd}
                        onChange={(event) => onChange({ ...draft, yearEnd: event.target.value })}
                        placeholder="2018"
                        disabled={disabled}
                    />
                </div>
            </div>
        </div>
    )
}

/** Campos de un fitment de maquinaria: marca, tipo y modelo y/o serie. */
export function EquipmentFitmentFields({
    draft,
    onChange,
    disabled,
    allowNew = true,
}: {
    draft: EquipmentFitmentDraft
    onChange: (draft: EquipmentFitmentDraft) => void
    disabled?: boolean
    allowNew?: boolean
}) {
    const brands = useBrandOptions()
    const types = useEquipmentTypes()
    const ids = useId()

    return (
        <div className="flex flex-col gap-4">
            <CatalogSelect
                label="Marca"
                items={brands.data ?? []}
                value={draft.brand}
                onChange={(brand) => onChange({ ...draft, brand })}
                placeholder="— Selecciona una marca —"
                newLabel="Marca nueva"
                allowCreate={allowNew}
                loading={brands.isLoading}
                disabled={disabled}
            />

            <CatalogSelect
                label="Tipo de maquinaria"
                items={types.data ?? []}
                value={draft.type}
                onChange={(type) => onChange({ ...draft, type })}
                placeholder="— Selecciona un tipo —"
                newLabel="Tipo nuevo"
                allowCreate={allowNew}
                loading={types.isLoading}
                disabled={disabled}
            />

            <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor={`${ids}-model`}>Modelo</Label>
                    <Input
                        id={`${ids}-model`}
                        value={draft.model}
                        onChange={(event) => onChange({ ...draft, model: event.target.value })}
                        disabled={disabled}
                        maxLength={255}
                    />
                </div>
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor={`${ids}-serie`}>Serie</Label>
                    <Input
                        id={`${ids}-serie`}
                        value={draft.serie}
                        onChange={(event) => onChange({ ...draft, serie: event.target.value })}
                        disabled={disabled}
                        maxLength={255}
                    />
                </div>
            </div>
            <p className="-mt-2 text-xs text-muted-foreground">Indica al menos el modelo o la serie.</p>
        </div>
    )
}

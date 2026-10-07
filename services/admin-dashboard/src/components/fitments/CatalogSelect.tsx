import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { CatalogValue } from "@/lib/fitments"
import { PlusIcon } from "lucide-react"
import { useId, useState } from "react"

const NONE = "0"

/**
 * Selector de una fila de catálogo (marca, tipo de maquinaria) con la opción de
 * escribir una nueva. Elegir una existente deja `{ id, name }`; escribir una
 * nueva deja `{ id: null, name }` y el backend la crea (o reutiliza la que ya
 * tenga ese nombre).
 */
export function CatalogSelect({
    label,
    items,
    value,
    onChange,
    placeholder,
    newLabel,
    allowCreate = true,
    loading = false,
    disabled = false,
}: {
    label: string
    items: { id: number; name: string }[]
    value: CatalogValue
    onChange: (value: CatalogValue) => void
    placeholder: string
    /** Texto del botón que cambia a modo "escribir nueva", p. ej. "Marca nueva". */
    newLabel: string
    allowCreate?: boolean
    loading?: boolean
    disabled?: boolean
}) {
    const fieldId = useId()
    // Arranca en modo "nueva" solo si el valor inicial ya es un nombre sin id.
    const [creating, setCreating] = useState(value.id === null && value.name !== "")

    const selectItems = [
        { label: placeholder, value: NONE },
        ...items.map((item) => ({ label: item.name, value: String(item.id) })),
    ]

    if (creating) {
        return (
            <div className="flex flex-col gap-1.5">
                <Label htmlFor={fieldId}>{label}</Label>
                <div className="flex items-center gap-2">
                    <Input
                        id={fieldId}
                        value={value.name}
                        onChange={(event) => onChange({ id: null, name: event.target.value })}
                        placeholder={newLabel}
                        disabled={disabled}
                        maxLength={255}
                        autoFocus
                    />
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        disabled={disabled}
                        onClick={() => {
                            setCreating(false)
                            onChange({ id: null, name: "" })
                        }}
                    >
                        Elegir existente
                    </Button>
                </div>
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-1.5">
            <Label htmlFor={fieldId}>{label}</Label>
            <div className="flex items-center gap-2">
                <Select
                    value={value.id ? String(value.id) : NONE}
                    onValueChange={(next) => {
                        const item = items.find((candidate) => String(candidate.id) === next)
                        onChange(item ? { id: item.id, name: item.name } : { id: null, name: "" })
                    }}
                    items={selectItems}
                    disabled={disabled || loading}
                >
                    <SelectTrigger id={fieldId} className="w-full min-w-0 flex-1">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="start">
                        <SelectGroup>
                            {selectItems.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                    {item.label}
                                </SelectItem>
                            ))}
                        </SelectGroup>
                    </SelectContent>
                </Select>
                {allowCreate ? (
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        disabled={disabled}
                        onClick={() => {
                            setCreating(true)
                            onChange({ id: null, name: "" })
                        }}
                    >
                        <PlusIcon />
                        {newLabel}
                    </Button>
                ) : null}
            </div>
        </div>
    )
}

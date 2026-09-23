import { Badge } from "@/components/product-detail/ProductDetailPrimitives"
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
import { useDeleteProductAttribute, useSetProductAttribute } from "@/lib/product-detail"
import type { AttributeChecklistItem } from "@/lib/schemas/product-details"
import { useMemo, useState } from "react"

// Tabla de checklist de atributos (requerido/opcional + edición inline de
// valor) compartida por la sección Atributos y el selector de categoría de
// Sincronización (ver ADR 0005) — antes vivía solo en ProductAttributesSection.tsx.

const dataTypeInputType: Record<string, "text" | "number" | "date"> = {
    number: "number",
    date: "date",
    text: "text",
}

// Valor del <Select> para "sin valor" (base-ui no maneja bien el string vacío).
const NONE = "__none__"

export function AttributeChecklistTable({
    productId,
    items,
}: {
    productId?: string
    items: AttributeChecklistItem[]
}) {
    return (
        <Table containerClassName="overflow-x-auto">
            <TableHeader>
                <TableRow>
                    <TableHead>Atributo</TableHead>
                    <TableHead>Estado</TableHead>
                    <TableHead className="w-64">Valor</TableHead>
                    <TableHead />
                </TableRow>
            </TableHeader>
            <TableBody>
                {items.map((item) => (
                    <ChecklistRow key={item.attributeId} productId={productId} item={item} />
                ))}
            </TableBody>
        </Table>
    )
}

function ChecklistRow({ productId, item }: { productId?: string; item: AttributeChecklistItem }) {
    const setMut = useSetProductAttribute(productId)
    const delMut = useDeleteProductAttribute(productId)
    const isSelect = item.dataType === "enum" || item.dataType === "boolean"

    // Borrador: para enum guardamos el optionId como string; para booleano
    // "true"/"false"; para el resto el texto tal cual. NONE = sin valor.
    const initial = useMemo(() => {
        if (item.dataType === "enum") {
            return item.optionId != null ? String(item.optionId) : NONE
        }
        if (item.dataType === "boolean") {
            return item.value === "Sí" ? "true" : item.value === "No" ? "false" : NONE
        }
        return item.value
    }, [item])
    const [draft, setDraft] = useState(initial)
    const [error, setError] = useState<string | null>(null)
    // Valor que acabamos de mandar al servidor: la fila queda "guardando" (sin
    // botones clickeables) hasta que la lista recargada refleje ese valor —
    // solo afecta a ESTA fila, no a las demás.
    const [savedValue, setSavedValue] = useState<string | null>(null)

    const settling = savedValue !== null && savedValue !== initial
    const isSaving = setMut.isPending || delMut.isPending || settling
    const missing = item.isRequired && !item.value && item.optionId == null
    const isDirty = draft !== initial && !settling

    function clearValue() {
        setError(null)
        const cleared = isSelect ? NONE : ""
        if (item.productAttributeId == null) {
            setDraft(cleared)
            return
        }
        delMut.mutate(item.productAttributeId, {
            onSuccess: () => {
                setDraft(cleared)
                setSavedValue(cleared)
            },
            onError: () => setError("No se pudo limpiar."),
        })
    }

    function save() {
        setError(null)
        const trimmed = draft === NONE ? "" : draft.trim()

        if (trimmed === "") {
            clearValue()
            return
        }

        let payload: Parameters<typeof setMut.mutate>[0]
        let persisted = trimmed
        switch (item.dataType) {
            case "number": {
                const n = Number(trimmed)
                if (!Number.isFinite(n)) {
                    setError("Número inválido")
                    return
                }
                payload = { attributeId: item.attributeId, valueNumber: n }
                persisted = String(n)
                break
            }
            case "boolean":
                payload = { attributeId: item.attributeId, valueBoolean: trimmed === "true" }
                break
            case "date":
                payload = { attributeId: item.attributeId, valueDate: `${trimmed}T00:00:00Z` }
                break
            case "enum": {
                const id = Number(trimmed)
                if (!id) {
                    setError("Opción inválida")
                    return
                }
                payload = { attributeId: item.attributeId, optionId: id }
                break
            }
            default:
                payload = { attributeId: item.attributeId, valueText: trimmed }
        }

        setMut.mutate(payload, {
            onSuccess: () => {
                setDraft(persisted)
                setSavedValue(persisted)
            },
            onError: () => setError("No se pudo guardar."),
        })
    }

    const selectItems = isSelect
        ? [
              { label: "— Sin valor —", value: NONE },
              ...(item.dataType === "enum"
                  ? item.options.map((option) => ({ label: option.value, value: String(option.id) }))
                  : [
                        { label: "Sí", value: "true" },
                        { label: "No", value: "false" },
                    ]),
          ]
        : []

    const control = isSelect ? (
        <Select
            value={draft}
            onValueChange={(value) => setDraft(value ?? NONE)}
            items={selectItems}
            disabled={isSaving}
        >
            <SelectTrigger className="w-full" aria-label={item.name}>
                <SelectValue />
            </SelectTrigger>
            <SelectContent align="start">
                <SelectGroup>
                    {selectItems.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                            {option.label}
                        </SelectItem>
                    ))}
                </SelectGroup>
            </SelectContent>
        </Select>
    ) : (
        <Input
            type={dataTypeInputType[item.dataType] ?? "text"}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            disabled={isSaving}
            aria-label={item.name}
            aria-invalid={error ? true : undefined}
        />
    )

    return (
        <TableRow>
            <TableCell>
                <span className="font-medium">{item.name}</span>
                {item.externalLabel && item.externalLabel !== item.name ? (
                    <span className="ml-1 text-xs text-muted-foreground">({item.externalLabel})</span>
                ) : null}
            </TableCell>
            <TableCell>
                {item.isRequired ? (
                    <Badge tone={missing ? "warning" : "success"}>{missing ? "Falta" : "Requerido"}</Badge>
                ) : (
                    <Badge tone="muted">Opcional</Badge>
                )}
            </TableCell>
            <TableCell>
                {control}
                {error ? <p className="mt-1 text-xs text-destructive">{error}</p> : null}
            </TableCell>
            <TableCell>
                <div className="flex items-center justify-end gap-1.5">
                    {isSaving ? (
                        <span className="text-xs text-muted-foreground">Guardando…</span>
                    ) : isDirty ? (
                        <>
                            <Button
                                variant="ghost"
                                size="xs"
                                onClick={() => {
                                    setDraft(initial)
                                    setError(null)
                                }}
                            >
                                Cancelar
                            </Button>
                            <Button size="xs" onClick={save}>
                                Guardar
                            </Button>
                        </>
                    ) : item.productAttributeId != null ? (
                        <Button variant="ghost" size="xs" onClick={clearValue}>
                            Limpiar
                        </Button>
                    ) : null}
                </div>
            </TableCell>
        </TableRow>
    )
}

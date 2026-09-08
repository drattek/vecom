import {
    Badge,
    Field,
    FieldError,
    SectionCard,
    SectionEditActions,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
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
import { Textarea } from "@/components/ui/textarea"
import {
    useAttributeChecklist,
    useCardEditor,
    useChannelConnections,
    useDeleteProductAttribute,
    useProductSection,
    useSetProductAttribute,
    useUpdateProductDimensions,
    useUpdateProductSeo,
} from "@/lib/product-detail"
import {
    productAttributesSectionSchema,
    productDimensionsPatchSchema,
    productSeoPatchSchema,
    type AttributeChecklistItem,
    type ProductAttributeValue,
    type ProductDimensions,
    type ProductDimensionsPatch,
    type ProductSeo,
    type ProductSeoPatch,
} from "@/lib/schemas/product-details"
import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"

const dataTypeLabels: Record<string, string> = {
    text: "Texto",
    number: "Número",
    boolean: "Booleano",
    date: "Fecha",
    enum: "Lista",
}

// Medidas editables de la card Dimensiones, en orden. El volumen NO está aquí:
// lo calcula el core como largo*ancho*alto (cm³) y se muestra aparte.
const editableDimensionFields = [
    { key: "weight", label: "Peso", unit: "kg" },
    { key: "length", label: "Largo", unit: "cm" },
    { key: "width", label: "Ancho", unit: "cm" },
    { key: "height", label: "Alto", unit: "cm" },
    { key: "diameter", label: "Diámetro", unit: "cm" },
] as const

/** Volumen de caja (cm³) a partir del borrador; "—" si falta alguna medida. */
function computeBoxVolume(length: string, width: string, height: string): string {
    const volume = Number(length) * Number(width) * Number(height)
    return Number.isFinite(volume) && volume > 0 ? volume.toFixed(2) : "—"
}

export function ProductAttributesSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "attributes", productAttributesSectionSchema)

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudieron cargar los atributos del producto." />
    }

    return (
        <div className="flex flex-col gap-4">
            <DimensionsCard productId={productId} dimensions={data.dimensions ?? null} />

            <SeoCard productId={productId} seo={data.seo ?? null} />

            <AttributesChecklistCard productId={productId} assigned={data.attributes} />
        </div>
    )
}

const STORAGE_KEY = "product-attr-checklist-connection"

function readStoredConnectionId(): number | null {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        const parsed = raw ? Number(raw) : NaN
        return Number.isFinite(parsed) && parsed > 0 ? parsed : null
    } catch {
        return null
    }
}

function AssignedAttributesTable({ rows }: { rows: ProductAttributeValue[] }) {
    return (
        <Table containerClassName="overflow-x-auto">
            <TableHeader>
                <TableRow>
                    <TableHead>Atributo</TableHead>
                    <TableHead>Código</TableHead>
                    <TableHead>Tipo</TableHead>
                    <TableHead>Valor</TableHead>
                    <TableHead>Actualizado por</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {rows.map((attribute) => (
                    <TableRow key={attribute.id}>
                        <TableCell>{attribute.name}</TableCell>
                        <TableCell className="text-muted-foreground">{attribute.code}</TableCell>
                        <TableCell className="text-muted-foreground">
                            {dataTypeLabels[attribute.dataType] ?? attribute.dataType}
                        </TableCell>
                        <TableCell className="font-medium">
                            {attribute.value || "—"}
                            {attribute.unit ? (
                                <span className="ml-1 font-normal text-muted-foreground">{attribute.unit}</span>
                            ) : null}
                        </TableCell>
                        <TableCell className="text-muted-foreground">{attribute.updatedBy ?? "—"}</TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    )
}

function AttributesChecklistCard({
    productId,
    assigned,
}: {
    productId?: string
    assigned: ProductAttributeValue[]
}) {
    const connections = useChannelConnections()
    const [pickedConnectionId, setPickedConnectionId] = useState<number | null>(readStoredConnectionId)

    // Si el usuario no eligió nada, usar la primera conexión disponible.
    const connectionId = pickedConnectionId ?? connections.data?.[0]?.id ?? null
    const checklist = useAttributeChecklist(productId, connectionId)

    function handleConnectionChange(value: string | null) {
        const id = Number(value)
        if (!Number.isFinite(id) || id <= 0) {
            return
        }
        setPickedConnectionId(id)
        try {
            localStorage.setItem(STORAGE_KEY, String(id))
        } catch {
            /* localStorage no disponible: se ignora */
        }
    }

    const selector =
        (connections.data ?? []).length > 0 ? (
            <Select
                value={connectionId != null ? String(connectionId) : ""}
                onValueChange={handleConnectionChange}
                items={(connections.data ?? []).map((connection) => ({
                    label: `${connection.channelName ?? "Canal"} · ${connection.name}`,
                    value: String(connection.id),
                }))}
            >
                <SelectTrigger className="min-w-52" aria-label="Conexión">
                    <SelectValue placeholder="Elegí una conexión" />
                </SelectTrigger>
                <SelectContent align="end">
                    <SelectGroup>
                        {(connections.data ?? []).map((connection) => (
                            <SelectItem key={connection.id} value={String(connection.id)}>
                                {connection.channelName ?? "Canal"} · {connection.name}
                            </SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
        ) : null

    // Atributos ya asignados que no están en el checklist del canal.
    const checklistAttrIds = new Set((checklist.data?.items ?? []).map((item) => item.attributeId))
    const otherAssigned = assigned.filter((attribute) => !checklistAttrIds.has(attribute.attributeId))

    return (
        <SectionCard
            title="Atributos"
            description="Atributos que el canal espera para la categoría del producto, y su estado."
            action={selector}
        >
            {connections.isLoading ? (
                <SectionState state="loading" />
            ) : (connections.data ?? []).length === 0 ? (
                <>
                    <p className="mb-3 text-sm text-muted-foreground">
                        No hay conexiones de canal configuradas. Se muestran los atributos ya asignados.
                    </p>
                    {assigned.length === 0 ? (
                        <SectionState state="empty" message="Este producto no tiene atributos asignados." />
                    ) : (
                        <AssignedAttributesTable rows={assigned} />
                    )}
                </>
            ) : checklist.isLoading ? (
                <SectionState state="loading" />
            ) : checklist.isError || !checklist.data ? (
                <SectionState state="error" message="No se pudo cargar el checklist de atributos." />
            ) : checklist.data.state === "no_category" ? (
                <SectionState
                    state="empty"
                    message="Asigná una categoría al producto (sección Clasificación) para ver los atributos que el canal requiere."
                />
            ) : checklist.data.state === "category_not_mapped" ? (
                <SectionState
                    state="empty"
                    message={`En ${checklist.data.channelName} la categoría «${checklist.data.categoryName ?? ""}» aún no tiene mapeo. Mapeala desde la configuración del canal para ver sus atributos.`}
                />
            ) : (
                <div className="flex flex-col gap-4">
                    {checklist.data.requiredMissing > 0 ? (
                        <p className="text-sm font-medium text-destructive">
                            {checklist.data.requiredMissing} atributo(s) requerido(s) sin completar
                        </p>
                    ) : (
                        <p className="text-sm text-muted-foreground">Todos los atributos requeridos están completos.</p>
                    )}

                    {checklist.data.items.length === 0 ? (
                        <SectionState state="empty" message="El canal no define atributos personalizados para esta categoría." />
                    ) : (
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
                                {checklist.data.items.map((item) => (
                                    <ChecklistRow key={item.attributeId} productId={productId} item={item} />
                                ))}
                            </TableBody>
                        </Table>
                    )}

                    {otherAssigned.length > 0 ? (
                        <div className="flex flex-col gap-2">
                            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                Otros atributos asignados
                            </p>
                            <AssignedAttributesTable rows={otherAssigned} />
                        </div>
                    ) : null}
                </div>
            )}
        </SectionCard>
    )
}

const dataTypeInputType: Record<string, "text" | "number" | "date"> = {
    number: "number",
    date: "date",
    text: "text",
}

// Valor del <Select> para "sin valor" (base-ui no maneja bien el string vacío).
const NONE = "__none__"

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

function DimensionsCard({
    productId,
    dimensions,
}: {
    productId?: string
    dimensions: ProductDimensions | null
}) {
    const mutation = useUpdateProductDimensions(productId)
    const editor = useCardEditor({
        seed: (): Record<string, string> =>
            Object.fromEntries(
                editableDimensionFields.map((field) => [field.key, dimensions?.[field.key] ?? "0"]),
            ),
        isSaving: mutation.isPending,
        submit: (patch: ProductDimensionsPatch) => mutation.mutateAsync(patch),
        onReset: () => mutation.reset(),
        fallbackErrorMessage: "No se pudieron guardar las dimensiones.",
        validate: (draft) => {
            const parsed = productDimensionsPatchSchema.safeParse(draft)
            if (parsed.success) {
                return { ok: true, patch: parsed.data }
            }
            const issue = parsed.error.issues[0]
            const label = editableDimensionFields.find((field) => field.key === issue?.path[0])?.label
            return { ok: false, message: label ? `${label}: ${issue?.message}` : (issue?.message ?? "Revisa los campos.") }
        },
    })

    return (
        <SectionCard
            title="Dimensiones"
            description="Medidas en centímetros y peso en kilogramos. El volumen se calcula como largo × ancho × alto."
            action={
                <SectionEditActions
                    isEditing={editor.isEditing}
                    isSaving={editor.isSaving}
                    onEdit={editor.start}
                    onCancel={editor.cancel}
                    onSave={editor.save}
                />
            }
        >
            {editor.isEditing ? (
                <>
                    <dl className="grid gap-4 sm:grid-cols-3 lg:grid-cols-6">
                        {editableDimensionFields.map((field) => (
                            <Field
                                key={field.key}
                                label={`${field.label} (${field.unit})`}
                                value={
                                    <Input
                                        type="number"
                                        min="0"
                                        step="0.01"
                                        inputMode="decimal"
                                        value={editor.draft[field.key] ?? ""}
                                        onChange={(event) =>
                                            editor.setDraft({ ...editor.draft, [field.key]: event.target.value })
                                        }
                                        disabled={editor.isSaving}
                                        aria-label={field.label}
                                    />
                                }
                            />
                        ))}
                        <Field
                            label="Volumen (cm³)"
                            value={
                                <span className="text-muted-foreground">
                                    {computeBoxVolume(
                                        editor.draft.length ?? "",
                                        editor.draft.width ?? "",
                                        editor.draft.height ?? "",
                                    )}{" "}
                                    <span className="text-xs">(calculado)</span>
                                </span>
                            }
                        />
                    </dl>
                    <FieldError message={editor.error} />
                </>
            ) : dimensions ? (
                <dl className="grid gap-4 sm:grid-cols-3 lg:grid-cols-6">
                    {editableDimensionFields.map((field) => (
                        <Field
                            key={field.key}
                            label={field.label}
                            value={`${dimensions[field.key]} ${field.unit}`}
                        />
                    ))}
                    <Field label="Volumen" value={`${dimensions.volume} cm³`} />
                </dl>
            ) : (
                <SectionState state="empty" message="Este producto no tiene dimensiones registradas." />
            )}
        </SectionCard>
    )
}

function SeoCard({ productId, seo }: { productId?: string; seo: ProductSeo | null }) {
    const mutation = useUpdateProductSeo(productId)
    const editor = useCardEditor({
        seed: (): ProductSeoPatch => ({
            metaTitle: seo?.metaTitle ?? "",
            metaDescription: seo?.metaDescription ?? "",
            keywords: seo?.keywords ?? "",
        }),
        isSaving: mutation.isPending,
        submit: (patch: ProductSeoPatch) => mutation.mutateAsync(patch),
        onReset: () => mutation.reset(),
        fallbackErrorMessage: "No se pudo guardar la información de SEO.",
        validate: (draft) => {
            const parsed = productSeoPatchSchema.safeParse(draft)
            if (parsed.success) {
                return { ok: true, patch: parsed.data }
            }
            return { ok: false, message: parsed.error.issues[0]?.message ?? "Revisa los campos." }
        },
    })

    return (
        <SectionCard
            title="SEO"
            description="Metadatos usados al publicar el producto."
            action={
                <SectionEditActions
                    isEditing={editor.isEditing}
                    isSaving={editor.isSaving}
                    onEdit={editor.start}
                    onCancel={editor.cancel}
                    onSave={editor.save}
                />
            }
        >
            {editor.isEditing ? (
                <>
                    <dl className="grid gap-4">
                        <Field
                            label="Meta título"
                            value={
                                <Input
                                    value={editor.draft.metaTitle}
                                    onChange={(event) =>
                                        editor.setDraft({ ...editor.draft, metaTitle: event.target.value })
                                    }
                                    disabled={editor.isSaving}
                                    aria-label="Meta título"
                                    maxLength={255}
                                />
                            }
                        />
                        <Field
                            label="Meta descripción"
                            value={
                                <Textarea
                                    value={editor.draft.metaDescription}
                                    onChange={(event) =>
                                        editor.setDraft({ ...editor.draft, metaDescription: event.target.value })
                                    }
                                    disabled={editor.isSaving}
                                    aria-label="Meta descripción"
                                    maxLength={255}
                                    rows={2}
                                />
                            }
                        />
                        <Field
                            label="Palabras clave"
                            value={
                                <Textarea
                                    value={editor.draft.keywords}
                                    onChange={(event) =>
                                        editor.setDraft({ ...editor.draft, keywords: event.target.value })
                                    }
                                    disabled={editor.isSaving}
                                    aria-label="Palabras clave"
                                    rows={2}
                                />
                            }
                        />
                    </dl>
                    <FieldError message={editor.error} />
                </>
            ) : seo && (seo.metaTitle || seo.metaDescription || seo.keywords) ? (
                <dl className="grid gap-4">
                    <Field label="Meta título" value={seo.metaTitle} />
                    <Field label="Meta descripción" value={seo.metaDescription} />
                    <Field label="Palabras clave" value={seo.keywords} />
                </dl>
            ) : (
                <SectionState state="empty" message="Este producto no tiene información de SEO." />
            )}
        </SectionCard>
    )
}

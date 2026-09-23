import {
    Field,
    FieldError,
    SectionCard,
    SectionEditActions,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
import { AttributeChecklistTable } from "@/components/product-detail/AttributeChecklistTable"
import { ExternalCategoryTree } from "@/components/categories/ExternalCategoryTree"
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
import { getServerErrorMessage } from "@/lib/api"
import {
    useAddChannelAttributeValue,
    useAttributeChecklist,
    useCardEditor,
    useCategoryPrediction,
    useChannelConnections,
    useProductSection,
    useSelectChannelCategory,
    useUpdateProductDimensions,
    useUpdateProductSeo,
} from "@/lib/product-detail"
import {
    productAttributesSectionSchema,
    productDimensionsPatchSchema,
    productGeneralSchema,
    productSeoPatchSchema,
    type ProductAttributeValue,
    type ProductDimensions,
    type ProductDimensionsPatch,
    type ProductSeo,
    type ProductSeoPatch,
} from "@/lib/schemas/product-details"
import { useState } from "react"
import { useParams } from "react-router-dom"

// ecom_channels.code para MercadoLibre — la única conexión que tiene
// predictor de categoría por texto (ver ADR 0005). Cualquier otro canal
// soportado (hoy solo Odoo) muestra el árbol de categorías directamente.
const MERCADOLIBRE_CHANNEL_CODE = "MERCADOLIBRE"

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
    const general = useProductSection(productId, "general", productGeneralSchema)
    const [pickedConnectionId, setPickedConnectionId] = useState<number | null>(readStoredConnectionId)

    // Si el usuario no eligió nada, usar la primera conexión disponible.
    const connectionId = pickedConnectionId ?? connections.data?.[0]?.id ?? null
    const selectedConnection = (connections.data ?? []).find((connection) => connection.id === connectionId)
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
            ) : checklist.data.state === "needs_selection" && connectionId != null ? (
                <div className="flex flex-col gap-3">
                    <p className="text-sm text-muted-foreground">
                        Elegí la categoría de {checklist.data.channelName} para esta conexión — es lo que habilita
                        publicar desde Sincronización.
                    </p>
                    <CategorySelector
                        productId={productId}
                        connectionId={connectionId}
                        channelCode={selectedConnection?.channelCode ?? ""}
                        productName={general.data?.name}
                    />
                </div>
            ) : (
                <div className="flex flex-col gap-4">
                    {checklist.data.attributeScope === "product" ? (
                        <p className="text-sm text-muted-foreground">
                            Atributos de este producto para {checklist.data.channelName}. Agregá los que necesites — cada
                            producto tiene los suyos.
                        </p>
                    ) : checklist.data.requiredMissing > 0 ? (
                        <p className="text-sm font-medium text-destructive">
                            {checklist.data.requiredMissing} atributo(s) requerido(s) sin completar
                        </p>
                    ) : (
                        <p className="text-sm text-muted-foreground">Todos los atributos requeridos están completos.</p>
                    )}

                    {checklist.data.items.length === 0 ? (
                        <SectionState
                            state="empty"
                            message={
                                checklist.data.attributeScope === "product"
                                    ? "Este producto todavía no tiene atributos. Agregá el primero abajo."
                                    : "El canal no define atributos personalizados aplicables a este producto."
                            }
                        />
                    ) : (
                        <AttributeChecklistTable productId={productId} items={checklist.data.items} />
                    )}

                    {checklist.data.attributeScope === "product" && connectionId != null && general.data?.sku ? (
                        <AddCustomAttributeForm
                            productId={productId}
                            sku={general.data.sku}
                            connectionId={connectionId}
                        />
                    ) : null}

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

/**
 * Selector de categoría externa por conexión, previo a publicar (ver ADR
 * 0005). MercadoLibre muestra primero la sugerencia del predictor de
 * categoría (por nombre de producto), con opción de elegir otra desde el
 * árbol completo; cualquier otro canal soportado (hoy Odoo, que no tiene
 * predictor) muestra el árbol directamente. Vive en Atributos (no en
 * Sincronización) porque es lo que determina qué atributos requeridos
 * aplican; Sincronización solo lee el resultado para habilitar "Publicar".
 */
function CategorySelector({
    productId,
    connectionId,
    channelCode,
    productName,
}: {
    productId?: string
    connectionId: number
    channelCode: string
    productName?: string
}) {
    const isMercadoLibre = channelCode === MERCADOLIBRE_CHANNEL_CODE
    const [showTree, setShowTree] = useState(!isMercadoLibre)
    const prediction = useCategoryPrediction(isMercadoLibre ? connectionId : null, isMercadoLibre ? productName : undefined)
    const mutation = useSelectChannelCategory(productId)
    const [pickingExternalId, setPickingExternalId] = useState<string | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    function pick(externalCategoryId: string) {
        setErrorMessage(null)
        setPickingExternalId(externalCategoryId)
        mutation.mutate(
            { connectionId, externalCategoryId },
            {
                onError: (error) => {
                    setErrorMessage(getServerErrorMessage(error, "No se pudo guardar la categoría."))
                    setPickingExternalId(null)
                },
            },
        )
    }

    if (isMercadoLibre && !showTree) {
        return (
            <div className="flex flex-col gap-2">
                {prediction.isLoading ? (
                    <p className="text-sm text-muted-foreground">Buscando categoría sugerida…</p>
                ) : prediction.data ? (
                    <div className="flex flex-col gap-2 rounded-lg border border-border p-3">
                        <p className="text-sm">
                            Categoría sugerida: <span className="font-medium">{prediction.data.category_name}</span>{" "}
                            <span className="text-xs text-muted-foreground">({prediction.data.category_id})</span>
                        </p>
                        <div className="flex gap-2">
                            <Button size="sm" disabled={mutation.isPending} onClick={() => pick(prediction.data!.category_id)}>
                                {mutation.isPending ? "Guardando…" : "Usar esta categoría"}
                            </Button>
                            <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => setShowTree(true)}>
                                Elegir otra
                            </Button>
                        </div>
                    </div>
                ) : (
                    <div className="flex flex-col items-start gap-2">
                        <p className="text-sm text-muted-foreground">No se pudo sugerir una categoría automáticamente.</p>
                        <Button size="sm" variant="outline" onClick={() => setShowTree(true)}>
                            Elegir categoría
                        </Button>
                    </div>
                )}
                <FieldError message={errorMessage} />
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-2">
            <ExternalCategoryTree
                connectionId={connectionId}
                pickLabel="Seleccionar"
                pickingExternalId={mutation.isPending ? pickingExternalId : null}
                busy={mutation.isPending}
                onPick={(node) => pick(node.externalId)}
            />
            <FieldError message={errorMessage} />
        </div>
    )
}

const addAttributeDataTypes = [
    { value: "text", label: "Texto" },
    { value: "number", label: "Número" },
    { value: "boolean", label: "Booleano" },
    { value: "date", label: "Fecha" },
] as const

type AddAttributeDataType = (typeof addAttributeDataTypes)[number]["value"]

// Sentinel para "sin elegir" en el <Select> booleano (base-ui no maneja bien "").
const PICK_NONE = "__pick_none__"

const addAttributeBooleanItems = [
    { label: "— Elegí —", value: PICK_NONE },
    { label: "Sí", value: "true" },
    { label: "No", value: "false" },
]

/**
 * Alta libre de un atributo para canales `attributeScope = 'product'` (Odoo):
 * clave + tipo + valor. Crea toda la cadena (atributo, slot dynamic_field,
 * mapeo, valor) en una sola llamada y el checklist se refresca solo.
 */
function AddCustomAttributeForm({
    productId,
    sku,
    connectionId,
}: {
    productId?: string
    sku: string
    connectionId: number
}) {
    const mutation = useAddChannelAttributeValue(productId)
    const [key, setKey] = useState("")
    const [dataType, setDataType] = useState<AddAttributeDataType>("text")
    const [value, setValue] = useState("")
    const [error, setError] = useState<string | null>(null)

    const trimmedKey = key.trim()
    const trimmedValue = value.trim()
    const canSubmit =
        trimmedKey !== "" && (dataType === "boolean" ? value !== "" : trimmedValue !== "") && !mutation.isPending

    function submit() {
        setError(null)

        let typedValue: string | number | boolean
        switch (dataType) {
            case "number": {
                const parsed = Number(trimmedValue)
                if (!Number.isFinite(parsed)) {
                    setError("Número inválido")
                    return
                }
                typedValue = parsed
                break
            }
            case "boolean":
                typedValue = value === "true"
                break
            default:
                typedValue = trimmedValue
        }

        mutation.mutate(
            { sku, connectionId, externalKey: trimmedKey, dataType, value: typedValue },
            {
                onSuccess: () => {
                    setKey("")
                    setValue("")
                    setDataType("text")
                },
                onError: (err) => setError(getServerErrorMessage(err, "No se pudo agregar el atributo.")),
            },
        )
    }

    const valueControl =
        dataType === "boolean" ? (
            <Select
                value={value === "" ? PICK_NONE : value}
                onValueChange={(next) => setValue(next === PICK_NONE ? "" : (next ?? ""))}
                items={addAttributeBooleanItems}
                disabled={mutation.isPending}
            >
                <SelectTrigger className="w-full" aria-label="Valor">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent align="start">
                    <SelectGroup>
                        {addAttributeBooleanItems.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                                {option.label}
                            </SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
        ) : (
            <Input
                type={dataType === "number" ? "number" : dataType === "date" ? "date" : "text"}
                value={value}
                onChange={(event) => setValue(event.target.value)}
                disabled={mutation.isPending}
                aria-label="Valor"
                placeholder="Valor"
            />
        )

    return (
        <div className="flex flex-col gap-3 rounded-lg border border-dashed border-border p-3">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Agregar atributo</p>
            <div className="grid gap-3 sm:grid-cols-[1fr_10rem_1fr_auto] sm:items-end">
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Clave
                    <Input
                        value={key}
                        onChange={(event) => setKey(event.target.value)}
                        disabled={mutation.isPending}
                        aria-label="Clave del atributo"
                        placeholder="Ej. Material"
                    />
                </label>
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Tipo
                    <Select
                        value={dataType}
                        onValueChange={(next) => {
                            setDataType((next as AddAttributeDataType) ?? "text")
                            setValue("")
                        }}
                        items={addAttributeDataTypes.map((option) => ({ label: option.label, value: option.value }))}
                        disabled={mutation.isPending}
                    >
                        <SelectTrigger className="w-full" aria-label="Tipo">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent align="start">
                            <SelectGroup>
                                {addAttributeDataTypes.map((option) => (
                                    <SelectItem key={option.value} value={option.value}>
                                        {option.label}
                                    </SelectItem>
                                ))}
                            </SelectGroup>
                        </SelectContent>
                    </Select>
                </label>
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Valor
                    {valueControl}
                </label>
                <Button size="sm" onClick={submit} disabled={!canSubmit}>
                    {mutation.isPending ? "Agregando…" : "Agregar"}
                </Button>
            </div>
            <FieldError message={error} />
        </div>
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

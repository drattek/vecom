import { CategoryPickerDialog } from "@/components/product-detail/CategoryPickerDialog"
import {
    Badge,
    Field,
    FieldError,
    SectionCard,
    SectionEditActions,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import {
    formatDate,
    useBrandOptions,
    useCardEditor,
    useProductSection,
    useUpdateProductGeneral,
    type CardEditorValidation,
} from "@/lib/product-detail"
import {
    productGeneralPatchSchema,
    productGeneralSchema,
    type ProductGeneral,
    type ProductGeneralPatch,
} from "@/lib/schemas/product-details"
import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"

// NONE_VALUE es el valor del <Select> para "sin marca"; el PATCH del core
// interpreta 0 como "limpiar la relación".
const NONE_VALUE = "0"

function validateGeneralDraft(
    draft: Partial<ProductGeneralPatch>,
): CardEditorValidation<ProductGeneralPatch> {
    const parsed = productGeneralPatchSchema.safeParse(draft)
    if (parsed.success) {
        return { ok: true, patch: parsed.data }
    }
    return { ok: false, message: parsed.error.issues[0]?.message ?? "Revisa los campos." }
}

const productTypeLabels: Record<string, string> = {
    part: "Refacción",
    consumable: "Consumible",
    accessory: "Accesorio",
}

const statusLabels: Record<string, string> = {
    active: "Activo",
    discontinued: "Descontinuado",
    hidden: "Oculto",
}

export function ProductGeneralSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "general", productGeneralSchema)

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudo cargar la información del producto." />
    }

    return (
        <div className="flex flex-col gap-4">
            <IdentificationCard productId={productId} data={data} />
            <ClassificationCard productId={productId} data={data} />
            <DescriptionCard productId={productId} data={data} />

            <SectionCard title="Auditoría">
                <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                    <Field label="Creado" value={formatDate(data.createdAt, true)} />
                    <Field label="Creado por" value={data.createdBy} />
                    <Field label="Actualizado" value={formatDate(data.updatedAt, true)} />
                    <Field label="Actualizado por" value={data.updatedBy} />
                </dl>
            </SectionCard>
        </div>
    )
}

function IdentificationCard({ productId, data }: { productId?: string; data: ProductGeneral }) {
    const [imageErrored, setImageErrored] = useState(false)
    const mutation = useUpdateProductGeneral(productId)
    const editor = useCardEditor({
        seed: () => ({ name: data.name }),
        isSaving: mutation.isPending,
        validate: validateGeneralDraft,
        submit: (patch) => mutation.mutateAsync(patch),
        onReset: () => mutation.reset(),
    })

    return (
        <SectionCard
            title="Identificación"
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
            <div className="flex flex-col gap-6 sm:flex-row">
                <div className="shrink-0">
                    {data.coverImage && !imageErrored ? (
                        <img
                            src={data.coverImage}
                            alt={data.name}
                            onError={() => setImageErrored(true)}
                            className="size-32 rounded-md border border-border object-cover"
                        />
                    ) : (
                        <div className="flex size-32 items-center justify-center rounded-md border border-dashed border-border text-xs text-muted-foreground">
                            Sin imagen
                        </div>
                    )}
                </div>

                <dl className="grid min-w-0 flex-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                    <Field label="SKU" value={<span className="font-medium">{data.sku}</span>} />
                    <Field label="Número de parte" value={data.partNumber} />
                    <Field label="Tipo" value={productTypeLabels[data.productType] ?? data.productType} />
                    <Field
                        label="Estatus"
                        value={
                            data.status ? (
                                <Badge tone={data.status === "active" ? "success" : "muted"}>
                                    {statusLabels[data.status] ?? data.status}
                                </Badge>
                            ) : null
                        }
                    />
                    <Field
                        label="Vendible"
                        value={<Badge tone={data.isSellable ? "success" : "muted"}>{data.isSellable ? "Sí" : "No"}</Badge>}
                    />
                    <Field
                        label="Inventariable"
                        value={<Badge tone={data.isStockable ? "success" : "muted"}>{data.isStockable ? "Sí" : "No"}</Badge>}
                    />
                    <Field
                        label="Nombre"
                        className="sm:col-span-2 lg:col-span-3"
                        value={
                            editor.isEditing ? (
                                <div>
                                    <Input
                                        value={editor.draft.name ?? ""}
                                        onChange={(event) => editor.setDraft({ name: event.target.value })}
                                        disabled={editor.isSaving}
                                        aria-invalid={editor.error ? true : undefined}
                                        aria-label="Nombre del producto"
                                        autoFocus
                                        maxLength={255}
                                    />
                                    <FieldError message={editor.error} />
                                </div>
                            ) : (
                                data.name
                            )
                        }
                    />
                </dl>
            </div>
        </SectionCard>
    )
}

function ClassificationCard({ productId, data }: { productId?: string; data: ProductGeneral }) {
    const mutation = useUpdateProductGeneral(productId)
    const editor = useCardEditor({
        seed: () => ({ brandId: data.brandId ?? 0, categoryId: data.categoryId ?? 0 }),
        isSaving: mutation.isPending,
        validate: validateGeneralDraft,
        submit: (patch) => mutation.mutateAsync(patch),
        onReset: () => mutation.reset(),
    })
    const brandOptions = useBrandOptions(editor.isEditing)

    // `items` alimenta a <SelectValue> para que muestre el nombre y no el id.
    // La marca actual se incluye como fallback por si las opciones aún cargan.
    const brandItems = useMemo(() => {
        const items: { label: string; value: string }[] = [{ label: "Sin marca", value: NONE_VALUE }]
        const seen = new Set([NONE_VALUE])
        if (data.brandId && data.brandName) {
            items.push({ label: data.brandName, value: String(data.brandId) })
            seen.add(String(data.brandId))
        }
        for (const brand of brandOptions.data ?? []) {
            const value = String(brand.id)
            if (!seen.has(value)) {
                items.push({ label: brand.name, value })
                seen.add(value)
            }
        }
        return items
    }, [brandOptions.data, data.brandId, data.brandName])

    return (
        <SectionCard
            title="Clasificación"
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
            <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <Field
                    label="Marca"
                    value={
                        editor.isEditing ? (
                            <Select
                                items={brandItems}
                                value={String(editor.draft.brandId ?? 0)}
                                onValueChange={(value) =>
                                    editor.setDraft({ ...editor.draft, brandId: Number(value ?? NONE_VALUE) })
                                }
                                disabled={editor.isSaving || brandOptions.isLoading}
                            >
                                <SelectTrigger className="w-full" aria-label="Marca">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent align="start">
                                    <SelectGroup>
                                        {brandItems.map((item) => (
                                            <SelectItem key={item.value} value={item.value}>
                                                {item.label}
                                            </SelectItem>
                                        ))}
                                    </SelectGroup>
                                </SelectContent>
                            </Select>
                        ) : (
                            data.brandName
                        )
                    }
                />
                <Field
                    label="Categoría"
                    value={
                        editor.isEditing ? (
                            <CategoryPickerDialog
                                value={editor.draft.categoryId ?? 0}
                                onChange={(categoryId) =>
                                    editor.setDraft({ ...editor.draft, categoryId })
                                }
                                disabled={editor.isSaving}
                            />
                        ) : (
                            data.categoryName
                        )
                    }
                />
                <Field label="Origen" value={data.sourceName} />
                <Field
                    label="Ruta de categoría"
                    value={data.categoryPath}
                    className="sm:col-span-2 lg:col-span-3"
                />
            </dl>
            <FieldError message={editor.error} />
        </SectionCard>
    )
}

function DescriptionCard({ productId, data }: { productId?: string; data: ProductGeneral }) {
    const mutation = useUpdateProductGeneral(productId)
    const editor = useCardEditor({
        seed: () => ({
            shortDescription: data.shortDescription ?? "",
            description: data.description ?? "",
        }),
        isSaving: mutation.isPending,
        validate: validateGeneralDraft,
        submit: (patch) => mutation.mutateAsync(patch),
        onReset: () => mutation.reset(),
    })

    return (
        <SectionCard
            title="Descripción"
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
            <dl className="grid gap-4">
                <Field
                    label="Descripción corta"
                    value={
                        editor.isEditing ? (
                            <Input
                                value={editor.draft.shortDescription ?? ""}
                                onChange={(event) =>
                                    editor.setDraft({ ...editor.draft, shortDescription: event.target.value })
                                }
                                disabled={editor.isSaving}
                                aria-label="Descripción corta"
                                maxLength={255}
                            />
                        ) : (
                            data.shortDescription
                        )
                    }
                />
                <Field
                    label="Descripción"
                    value={
                        editor.isEditing ? (
                            <Textarea
                                value={editor.draft.description ?? ""}
                                onChange={(event) =>
                                    editor.setDraft({ ...editor.draft, description: event.target.value })
                                }
                                disabled={editor.isSaving}
                                aria-label="Descripción"
                                rows={5}
                            />
                        ) : data.description ? (
                            <span className="whitespace-pre-wrap">{data.description}</span>
                        ) : null
                    }
                />
            </dl>
            <FieldError message={editor.error} />
        </SectionCard>
    )
}

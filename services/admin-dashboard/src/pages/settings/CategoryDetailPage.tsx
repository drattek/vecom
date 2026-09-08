import { CategoryTreePicker } from "@/components/categories/CategoryTreePicker"
import { LinkCategoryMappingDialog } from "@/components/categories/LinkCategoryMappingDialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import { IMPORT_SUPPORTED_CHANNEL_CODES, useCategoryTree, type CategoryNode } from "@/lib/categories"
import {
    categoryChannelMappingsResponseSchema,
    categoryDetailSchema,
    type CategoryChannelMapping,
} from "@/lib/schemas/categories"
import { cn } from "@/lib/utils"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeftIcon, PencilIcon } from "lucide-react"
import { useState } from "react"
import { Link, useParams } from "react-router-dom"

export function CategoryDetailPage() {
    const { categoryId } = useParams<{ categoryId: string }>()

    const categoryQuery = useQuery({
        queryKey: ["category", categoryId],
        enabled: Boolean(categoryId),
        queryFn: async () => {
            const response = await apiClient.get(`/api/categories/${categoryId}`)
            return categoryDetailSchema.parse(response.data)
        },
    })

    const treeQuery = useCategoryTree()

    const mappingsQuery = useQuery({
        queryKey: ["category-channel-mappings", categoryId],
        enabled: Boolean(categoryId),
        queryFn: async () => {
            const response = await apiClient.get(`/api/categories/${categoryId}/channel-mappings`)
            return categoryChannelMappingsResponseSchema.parse(response.data)
        },
    })

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <div className="flex items-center gap-2">
                <Link
                    to="/settings/marketplace-categories"
                    aria-label="Volver a categorías"
                    title="Volver a categorías"
                    className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                >
                    <ArrowLeftIcon className="size-4" />
                </Link>
                <h2 className="text-xl font-semibold text-foreground">
                    {categoryQuery.isError
                        ? "Categoría no encontrada"
                        : (categoryQuery.data?.name ?? "Cargando…")}
                </h2>
            </div>

            {categoryQuery.isError ? (
                <p className="mt-2 text-sm text-destructive">No se pudo cargar la categoría.</p>
            ) : null}

            <div className="mt-4 flex min-h-0 flex-1 flex-col gap-4 overflow-auto pb-4">
                {categoryQuery.data ? (
                    <GeneralCard category={categoryQuery.data} parentById={treeQuery.data?.byId} />
                ) : null}

                <section className="rounded-lg border border-border bg-card">
                    <header className="border-b border-border px-4 py-3">
                        <h3 className="text-sm font-semibold text-foreground">Mapeos por conexión</h3>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                            Categoría externa a la que corresponde en cada conexión de canal (solo lectura).
                        </p>
                    </header>
                    <div className="flex flex-col gap-3 p-4">
                        {mappingsQuery.isLoading ? (
                            <p className="py-4 text-center text-sm text-muted-foreground">Cargando conexiones…</p>
                        ) : null}
                        {mappingsQuery.isError ? (
                            <p className="py-4 text-center text-sm text-destructive">
                                No se pudieron cargar los mapeos.
                            </p>
                        ) : null}
                        {mappingsQuery.data && mappingsQuery.data.mappings.length === 0 ? (
                            <p className="py-4 text-center text-sm text-muted-foreground">
                                No hay conexiones de canal activas.
                            </p>
                        ) : null}
                        {mappingsQuery.data?.mappings.map((mapping) => (
                            <ConnectionMappingCard
                                key={mapping.connectionId}
                                categoryId={categoryId}
                                mapping={mapping}
                            />
                        ))}
                    </div>
                </section>
            </div>
        </section>
    )
}

function GeneralCard({
    category,
    parentById,
}: {
    category: { id: number; name: string; parentId?: number | null }
    parentById?: Map<number, CategoryNode>
}) {
    const queryClient = useQueryClient()
    const [isEditing, setIsEditing] = useState(false)
    const [name, setName] = useState(category.name)
    const [parentId, setParentId] = useState<number | null>(category.parentId ?? null)
    const [error, setError] = useState<string | null>(null)

    const mutation = useMutation({
        mutationFn: async (input: { name: string; parentId: number | null }) => {
            const response = await apiClient.put(`/api/categories/${category.id}`, input)
            return categoryDetailSchema.parse(response.data)
        },
        onSuccess: (data) => {
            queryClient.setQueryData(["category", String(category.id)], data)
            queryClient.invalidateQueries({ queryKey: ["categories"] })
            queryClient.invalidateQueries({ queryKey: ["category-tree"] })
        },
    })

    function start() {
        setName(category.name)
        setParentId(category.parentId ?? null)
        setError(null)
        mutation.reset()
        setIsEditing(true)
    }

    function cancel() {
        setIsEditing(false)
        setError(null)
    }

    async function save() {
        const trimmed = name.trim()
        if (trimmed === "") {
            setError("El nombre es obligatorio.")
            return
        }
        setError(null)
        try {
            await mutation.mutateAsync({ name: trimmed, parentId })
            setIsEditing(false)
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudieron guardar los cambios."))
        }
    }

    const currentParentLabel =
        category.parentId != null
            ? (parentById?.get(category.parentId)?.path ?? `Categoría #${category.parentId}`)
            : "Sin categoría padre (raíz)"

    return (
        <section className="rounded-lg border border-border bg-card">
            <header className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
                <h3 className="text-sm font-semibold text-foreground">General</h3>
                {isEditing ? (
                    <div className="flex items-center gap-1.5">
                        <Button variant="ghost" size="sm" onClick={cancel} disabled={mutation.isPending}>
                            Cancelar
                        </Button>
                        <Button size="sm" onClick={save} disabled={mutation.isPending}>
                            {mutation.isPending ? "Guardando…" : "Guardar"}
                        </Button>
                    </div>
                ) : (
                    <Button variant="ghost" size="sm" onClick={start}>
                        <PencilIcon />
                        Editar
                    </Button>
                )}
            </header>
            <div className="p-4">
                <dl className="grid gap-4 sm:grid-cols-2">
                    <Field
                        label="Nombre"
                        value={
                            isEditing ? (
                                <Input
                                    value={name}
                                    onChange={(event) => setName(event.target.value)}
                                    disabled={mutation.isPending}
                                    aria-invalid={error ? true : undefined}
                                    aria-label="Nombre de la categoría"
                                    autoFocus
                                    maxLength={255}
                                />
                            ) : (
                                category.name
                            )
                        }
                    />
                    <Field
                        label="Categoría padre"
                        value={
                            isEditing ? (
                                <CategoryTreePicker
                                    value={parentId}
                                    onChange={setParentId}
                                    excludeId={category.id}
                                    disabled={mutation.isPending}
                                    rootLabel="Sin categoría padre (raíz)"
                                />
                            ) : (
                                currentParentLabel
                            )
                        }
                    />
                </dl>
                {error ? (
                    <p className="mt-2 text-xs text-destructive" role="alert">
                        {error}
                    </p>
                ) : null}
            </div>
        </section>
    )
}

function ConnectionMappingCard({
    categoryId,
    mapping,
}: {
    categoryId: string | undefined
    mapping: CategoryChannelMapping
}) {
    const environmentLabel = mapping.environment === "production" ? "Producción" : "Desarrollo"
    const isMapped = Boolean(mapping.externalCategoryId)
    const canLink = IMPORT_SUPPORTED_CHANNEL_CODES.has(mapping.channelCode)

    return (
        <section className="rounded-lg border border-border bg-background">
            <header className="flex items-center justify-between gap-3 border-b border-border px-3 py-2">
                <h4 className="text-sm font-medium text-foreground">
                    {mapping.channelName} · {mapping.connectionName}
                </h4>
                <div className="flex items-center gap-2">
                    {canLink && isMapped ? (
                        <LinkCategoryMappingDialog categoryId={categoryId} mapping={mapping} />
                    ) : null}
                    <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                        {environmentLabel}
                    </span>
                </div>
            </header>
            <div className="p-3">
                {isMapped ? (
                    <dl className="grid gap-4 sm:grid-cols-3">
                        <Field label="ID externo" value={<span className="font-medium">{mapping.externalCategoryId}</span>} />
                        <Field label="Nombre externo" value={mapping.externalCategoryName} />
                        <Field label="Actualizado" value={formatDate(mapping.mappedAt)} />
                    </dl>
                ) : (
                    <div className="flex items-center justify-between gap-3">
                        <p className="text-sm text-muted-foreground">Sin mapeo para esta conexión.</p>
                        {canLink ? (
                            <LinkCategoryMappingDialog categoryId={categoryId} mapping={mapping} />
                        ) : null}
                    </div>
                )}
            </div>
        </section>
    )
}

function Field({ label, value }: { label: string; value?: React.ReactNode }) {
    const isEmpty = value === null || value === undefined || value === ""
    return (
        <div className="min-w-0">
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</dt>
            <dd className={cn("mt-1 text-sm wrap-break-word", isEmpty ? "text-muted-foreground" : "text-foreground")}>
                {isEmpty ? "—" : value}
            </dd>
        </div>
    )
}

function formatDate(value?: string | null): string {
    if (!value) {
        return "—"
    }
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
        return value
    }
    return date.toLocaleString("es-MX", {
        year: "numeric",
        month: "short",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
    })
}

import {
    Badge,
    FieldError,
    SectionCard,
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
import {
    formatDate,
    useAddAlternatePartNumber,
    useBrandOptions,
    useDeleteAlternatePartNumber,
    useProductSection,
} from "@/lib/product-detail"
import {
    productPartNumbersSectionSchema,
    type ProductAlternatePartNumber,
} from "@/lib/schemas/product-details"
import type { ProductDetailContext } from "@/pages/ProductDetailPage"
import { ArrowRightIcon, PencilIcon, PlusIcon, XIcon } from "lucide-react"
import { useState } from "react"
import { Link, useOutletContext, useParams } from "react-router-dom"

const partNumberTypeLabels: Record<string, string> = {
    aftermarket: "Aftermarket",
    cross_reference: "Referencia cruzada",
    supplier: "Proveedor",
    internal: "Interno",
}

const partNumberTypeOptions = Object.entries(partNumberTypeLabels).map(([value, label]) => ({ value, label }))

const NONE_BRAND = "0"

export function ProductPartNumbersSection() {
    const { productId } = useParams<{ productId: string }>()
    // Saltar a un producto relacionado conserva el origen, para que su botón de
    // regreso vuelva a la misma tabla y no a /products.
    const { originPath } = useOutletContext<ProductDetailContext>()
    const { data, isLoading, isError } = useProductSection(productId, "part-numbers", productPartNumbersSectionSchema)

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudieron cargar los números de parte." />
    }

    return (
        <div className="flex flex-col gap-4">
            <SectionCard title="Número de parte principal">
                <span className="text-lg font-semibold text-foreground">{data.partNumber}</span>
            </SectionCard>

            <AlternatePartNumbersCard productId={productId} alternates={data.alternates} />

            <SectionCard
                title="Sucesiones"
                description="Cadena de reemplazo reportada por el ERP: qué pieza sustituye a cuál."
                action={<span className="text-xs text-muted-foreground">{data.supersessions.length} registradas</span>}
            >
                {data.supersessions.length === 0 ? (
                    <SectionState state="empty" message="Este producto no participa en ninguna sucesión." />
                ) : (
                    <Table containerClassName="overflow-x-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Relación</TableHead>
                                <TableHead>Cadena</TableHead>
                                <TableHead>Producto relacionado</TableHead>
                                <TableHead>Estado</TableHead>
                                <TableHead>Registrada</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {data.supersessions.map((supersession) => {
                                const isSupersededBy = supersession.direction === "superseded_by"

                                return (
                                    <TableRow key={supersession.id}>
                                        <TableCell>
                                            <Badge tone={isSupersededBy ? "warning" : "neutral"}>
                                                {isSupersededBy ? "Reemplazado por" : "Reemplaza a"}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
                                                <span className="text-muted-foreground">{supersession.oldPartNumber}</span>
                                                <ArrowRightIcon className="size-3.5 text-muted-foreground" />
                                                <span className="font-medium">{supersession.newPartNumber}</span>
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            {supersession.otherProductId && supersession.otherProductSku ? (
                                                <Link
                                                    to={`/products/${supersession.otherProductId}`}
                                                    state={{ from: originPath }}
                                                    className="underline underline-offset-2 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                                                >
                                                    {supersession.otherProductSku}
                                                </Link>
                                            ) : (
                                                <span className="text-muted-foreground">
                                                    {supersession.otherPartNumber} (sin producto)
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            <Badge tone={supersession.resolved ? "success" : "muted"}>
                                                {supersession.resolved ? "Resuelta" : "Pendiente"}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="whitespace-nowrap text-muted-foreground">
                                            {formatDate(supersession.createdAt)}
                                        </TableCell>
                                    </TableRow>
                                )
                            })}
                        </TableBody>
                    </Table>
                )}
            </SectionCard>
        </div>
    )
}

function AlternatePartNumbersCard({
    productId,
    alternates,
}: {
    productId?: string
    alternates: ProductAlternatePartNumber[]
}) {
    const [isEditing, setIsEditing] = useState(false)
    const [partNumber, setPartNumber] = useState("")
    const [brandId, setBrandId] = useState(NONE_BRAND)
    const [type, setType] = useState("cross_reference")
    const [error, setError] = useState<string | null>(null)

    const addMut = useAddAlternatePartNumber(productId)
    const deleteMut = useDeleteAlternatePartNumber(productId)
    const brands = useBrandOptions(isEditing)
    const isBusy = addMut.isPending || deleteMut.isPending

    const brandItems = [
        { label: "— Marca —", value: NONE_BRAND },
        ...(brands.data ?? []).map((brand) => ({ label: brand.name, value: String(brand.id) })),
    ]

    function resetForm() {
        setPartNumber("")
        setBrandId(NONE_BRAND)
        setType("cross_reference")
        setError(null)
    }

    function add() {
        const trimmed = partNumber.trim()
        if (!trimmed) {
            setError("Ingresá un número de parte.")
            return
        }
        if (brandId === NONE_BRAND) {
            setError("Elegí una marca.")
            return
        }
        if (alternates.some((alternate) => alternate.partNumber.toLowerCase() === trimmed.toLowerCase())) {
            setError("Ese número de parte ya está registrado.")
            return
        }
        setError(null)
        addMut.mutate(
            { partNumber: trimmed, brandId: Number(brandId), type },
            {
                onSuccess: resetForm,
                onError: () => setError("No se pudo agregar el número de parte."),
            },
        )
    }

    function remove(alternatePartNumber: string) {
        setError(null)
        deleteMut.mutate(alternatePartNumber, {
            onError: () => setError("No se pudo quitar el número de parte."),
        })
    }

    return (
        <SectionCard
            title="Números de parte alternativos"
            description="Equivalencias y referencias cruzadas registradas para este producto."
            action={
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                        resetForm()
                        setIsEditing((value) => !value)
                    }}
                >
                    {isEditing ? (
                        "Listo"
                    ) : (
                        <>
                            <PencilIcon />
                            Editar
                        </>
                    )}
                </Button>
            }
        >
            {alternates.length === 0 && !isEditing ? (
                <SectionState state="empty" message="Este producto no tiene números de parte alternativos." />
            ) : (
                <div className="flex flex-col gap-3">
                    {alternates.length > 0 ? (
                        <div className="flex flex-wrap gap-2">
                            {alternates.map((alternate) => (
                                <div
                                    key={`${alternate.partNumber}-${alternate.brandId}`}
                                    className="min-w-36 overflow-hidden rounded-md border border-border bg-card text-xs"
                                >
                                    <div className="flex items-center justify-between gap-2 px-2.5 py-1.5">
                                        <span className="font-medium wrap-break-word text-foreground">
                                            {alternate.partNumber}
                                        </span>
                                        {isEditing ? (
                                            <button
                                                type="button"
                                                aria-label={`Quitar ${alternate.partNumber}`}
                                                disabled={isBusy}
                                                onClick={() => remove(alternate.partNumber)}
                                                className="-mr-0.5 shrink-0 rounded-sm text-muted-foreground outline-none hover:text-destructive focus-visible:ring-2 focus-visible:ring-ring/50 disabled:opacity-50"
                                            >
                                                <XIcon className="size-3.5" />
                                            </button>
                                        ) : null}
                                    </div>
                                    <div className="border-t border-border px-2.5 py-1.5 text-muted-foreground">
                                        {[
                                            alternate.brandName,
                                            partNumberTypeLabels[alternate.type] ?? alternate.type,
                                        ]
                                            .filter(Boolean)
                                            .join(" · ")}
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <p className="text-sm text-muted-foreground">Sin números de parte alternativos.</p>
                    )}

                    {isEditing ? (
                        <div className="flex flex-col gap-2 border-t border-border pt-3">
                            <div className="flex flex-wrap items-center gap-2">
                                <Input
                                    value={partNumber}
                                    onChange={(event) => setPartNumber(event.target.value)}
                                    placeholder="Número de parte"
                                    disabled={isBusy}
                                    className="w-44"
                                    aria-label="Número de parte"
                                    onKeyDown={(event) => {
                                        if (event.key === "Enter") {
                                            event.preventDefault()
                                            add()
                                        }
                                    }}
                                />
                                <Select
                                    value={brandId}
                                    onValueChange={(value) => setBrandId(value ?? NONE_BRAND)}
                                    items={brandItems}
                                    disabled={isBusy || brands.isLoading}
                                >
                                    <SelectTrigger className="w-40" aria-label="Marca">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent align="start">
                                        <SelectGroup>
                                            {brandItems.map((brand) => (
                                                <SelectItem key={brand.value} value={brand.value}>
                                                    {brand.label}
                                                </SelectItem>
                                            ))}
                                        </SelectGroup>
                                    </SelectContent>
                                </Select>
                                <Select
                                    value={type}
                                    onValueChange={(value) => setType(value ?? "cross_reference")}
                                    items={partNumberTypeOptions}
                                    disabled={isBusy}
                                >
                                    <SelectTrigger className="w-44" aria-label="Tipo">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent align="start">
                                        <SelectGroup>
                                            {partNumberTypeOptions.map((option) => (
                                                <SelectItem key={option.value} value={option.value}>
                                                    {option.label}
                                                </SelectItem>
                                            ))}
                                        </SelectGroup>
                                    </SelectContent>
                                </Select>
                                <Button size="sm" onClick={add} disabled={isBusy}>
                                    <PlusIcon />
                                    Agregar
                                </Button>
                            </div>
                            <FieldError message={error} />
                        </div>
                    ) : null}
                </div>
            )}
        </SectionCard>
    )
}

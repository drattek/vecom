import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { AddCompatibilityDialog } from "@/components/product-detail/AddCompatibilityDialog"
import { SectionCard, SectionState } from "@/components/product-detail/ProductDetailPrimitives"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import {
    formatYearRange,
    useUnlinkEquipmentCompatibility,
    useUnlinkVehicleCompatibility,
} from "@/lib/fitments"
import { useProductSection } from "@/lib/product-detail"
import {
    productCompatibilitiesSectionSchema,
    type ProductEquipmentCompatibility,
    type ProductVehicleCompatibility,
} from "@/lib/schemas/product-details"
import { PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"
import { useParams } from "react-router-dom"

/** Compatibilidad pendiente de confirmar su baja. */
type PendingRemoval =
    | { kind: "vehicle"; item: ProductVehicleCompatibility }
    | { kind: "equipment"; item: ProductEquipmentCompatibility }

function countLabel(count: number): string {
    return `${count} ${count === 1 ? "registrada" : "registradas"}`
}

function DashCell({ value }: { value?: string | null }) {
    return <TableCell className={value ? "" : "text-muted-foreground"}>{value || "—"}</TableCell>
}

export function ProductCompatibilitiesSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(
        productId,
        "compatibilities",
        productCompatibilitiesSectionSchema,
    )

    const [addOpen, setAddOpen] = useState(false)
    const [notice, setNotice] = useState<string | null>(null)
    const [removal, setRemoval] = useState<PendingRemoval | null>(null)
    const [removalError, setRemovalError] = useState<string | null>(null)

    const unlinkVehicle = useUnlinkVehicleCompatibility(productId)
    const unlinkEquipment = useUnlinkEquipmentCompatibility(productId)
    const isRemoving = unlinkVehicle.isPending || unlinkEquipment.isPending

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudieron cargar las compatibilidades." />
    }

    async function confirmRemoval() {
        if (!removal) {
            return
        }
        setRemovalError(null)
        try {
            if (removal.kind === "vehicle") {
                await unlinkVehicle.mutateAsync(removal.item.id)
            } else {
                await unlinkEquipment.mutateAsync(removal.item.id)
            }
            setNotice("Compatibilidad quitada del producto.")
            setRemoval(null)
        } catch (error) {
            setRemovalError(getServerErrorMessage(error, "No se pudo quitar la compatibilidad."))
        }
    }

    function askRemoval(next: PendingRemoval) {
        setRemovalError(null)
        setRemoval(next)
    }

    const removalLabel = removal
        ? removal.kind === "vehicle"
            ? `${removal.item.brandName ?? ""} ${removal.item.model} (${formatYearRange(removal.item.yearStart, removal.item.yearEnd)})`.trim()
            : [removal.item.brandName, removal.item.equipmentTypeName, removal.item.model, removal.item.serie]
                  .filter(Boolean)
                  .join(" · ")
        : ""

    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm text-muted-foreground">
                    Vehículos y maquinaria a los que aplica este producto.
                </p>
                <Button size="sm" onClick={() => setAddOpen(true)}>
                    <PlusIcon />
                    Agregar compatibilidad
                </Button>
            </div>

            {notice ? (
                <p
                    className="rounded-md border border-emerald-600/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-700 dark:text-emerald-400"
                    role="status"
                >
                    {notice}
                </p>
            ) : null}

            <SectionCard
                title="Compatibilidades de vehículo"
                description="Fitments a los que aplica este producto (marca, modelo y años) con sus calificadores de motor, posición y lado."
                action={<span className="text-xs text-muted-foreground">{countLabel(data.vehicles.length)}</span>}
            >
                {data.vehicles.length === 0 ? (
                    <SectionState state="empty" message="Este producto no tiene compatibilidades de vehículo." />
                ) : (
                    <Table containerClassName="overflow-x-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Marca</TableHead>
                                <TableHead>Modelo</TableHead>
                                <TableHead>Años</TableHead>
                                <TableHead>Motor</TableHead>
                                <TableHead>Posición</TableHead>
                                <TableHead>Lado</TableHead>
                                <TableHead className="w-12">
                                    <span className="sr-only">Acciones</span>
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {data.vehicles.map((compatibility) => (
                                <TableRow key={compatibility.id}>
                                    <TableCell>{compatibility.brandName ?? "—"}</TableCell>
                                    <TableCell>{compatibility.model}</TableCell>
                                    <TableCell className="whitespace-nowrap">
                                        {formatYearRange(compatibility.yearStart, compatibility.yearEnd)}
                                    </TableCell>
                                    <DashCell value={compatibility.motor} />
                                    <DashCell value={compatibility.position} />
                                    <DashCell value={compatibility.side} />
                                    <TableCell>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            aria-label={`Quitar ${compatibility.brandName ?? ""} ${compatibility.model}`}
                                            onClick={() => askRemoval({ kind: "vehicle", item: compatibility })}
                                        >
                                            <Trash2Icon />
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </SectionCard>

            <SectionCard
                title="Compatibilidades de maquinaria"
                description="Maquinaria a la que aplica este producto (marca, tipo, modelo y/o serie)."
                action={<span className="text-xs text-muted-foreground">{countLabel(data.equipment.length)}</span>}
            >
                {data.equipment.length === 0 ? (
                    <SectionState state="empty" message="Este producto no tiene compatibilidades de maquinaria." />
                ) : (
                    <Table containerClassName="overflow-x-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Marca</TableHead>
                                <TableHead>Tipo</TableHead>
                                <TableHead>Modelo</TableHead>
                                <TableHead>Serie</TableHead>
                                <TableHead className="w-12">
                                    <span className="sr-only">Acciones</span>
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {data.equipment.map((compatibility) => (
                                <TableRow key={compatibility.id}>
                                    <TableCell>{compatibility.brandName ?? "—"}</TableCell>
                                    <TableCell>{compatibility.equipmentTypeName ?? "—"}</TableCell>
                                    <DashCell value={compatibility.model} />
                                    <DashCell value={compatibility.serie} />
                                    <TableCell>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            aria-label={`Quitar ${compatibility.brandName ?? ""} ${compatibility.model ?? compatibility.serie ?? ""}`}
                                            onClick={() => askRemoval({ kind: "equipment", item: compatibility })}
                                        >
                                            <Trash2Icon />
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </SectionCard>

            <AddCompatibilityDialog
                productId={productId}
                open={addOpen}
                onOpenChange={setAddOpen}
                linkedVehicleFitmentIds={data.vehicles.map((compatibility) => compatibility.vehicleFitmentId)}
                linkedEquipmentFitmentIds={data.equipment.map((compatibility) => compatibility.equipmentFitmentId)}
                onAdded={setNotice}
            />

            <ConfirmDeleteDialog
                open={removal !== null}
                onOpenChange={(open) => {
                    if (!open && !isRemoving) {
                        setRemoval(null)
                    }
                }}
                title="Quitar compatibilidad"
                description={
                    <>
                        Se quitará <strong>{removalLabel}</strong> de este producto. El vehículo o la maquinaria siguen
                        existiendo en el catálogo.
                    </>
                }
                confirmLabel="Quitar"
                pending={isRemoving}
                error={removalError}
                onConfirm={() => void confirmRemoval()}
            />
        </div>
    )
}

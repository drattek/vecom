import { SectionCard, SectionState } from "@/components/product-detail/ProductDetailPrimitives"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useProductSection } from "@/lib/product-detail"
import {
    productCompatibilitiesSectionSchema,
    type ProductVehicleCompatibility,
} from "@/lib/schemas/product-details"
import { useParams } from "react-router-dom"

/** Rango de años del fitment: "2015" o "2015-2018". */
function yearRange(compatibility: ProductVehicleCompatibility): string {
    if (compatibility.yearEnd && compatibility.yearEnd !== compatibility.yearStart) {
        return `${compatibility.yearStart}-${compatibility.yearEnd}`
    }
    return String(compatibility.yearStart)
}

export function ProductCompatibilitiesSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(
        productId,
        "compatibilities",
        productCompatibilitiesSectionSchema,
    )

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudieron cargar las compatibilidades." />
    }

    return (
        <SectionCard
            title="Compatibilidades de vehículo"
            description="Fitments a los que aplica este producto (marca, modelo y años) con sus calificadores de motor, posición y lado."
            action={
                <span className="text-xs text-muted-foreground">
                    {data.vehicles.length} {data.vehicles.length === 1 ? "registrada" : "registradas"}
                </span>
            }
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
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {data.vehicles.map((compatibility) => (
                            <TableRow key={compatibility.id}>
                                <TableCell>{compatibility.brandName ?? "—"}</TableCell>
                                <TableCell>{compatibility.model}</TableCell>
                                <TableCell className="whitespace-nowrap">{yearRange(compatibility)}</TableCell>
                                <TableCell className={compatibility.motor ? "" : "text-muted-foreground"}>
                                    {compatibility.motor || "—"}
                                </TableCell>
                                <TableCell className={compatibility.position ? "" : "text-muted-foreground"}>
                                    {compatibility.position || "—"}
                                </TableCell>
                                <TableCell className={compatibility.side ? "" : "text-muted-foreground"}>
                                    {compatibility.side || "—"}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            )}
        </SectionCard>
    )
}

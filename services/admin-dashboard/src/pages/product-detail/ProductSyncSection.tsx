import { Badge, Field, SectionCard, SectionState } from "@/components/product-detail/ProductDetailPrimitives"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatDate, useProductSection } from "@/lib/product-detail"
import {
    productSyncSectionSchema,
    type SyncCompatibility,
    type SyncConnection,
    type SyncListing,
} from "@/lib/schemas/product-details"
import { useParams } from "react-router-dom"

// Estados reales de ecom_channel_product_map.status. "Pendiente" (más abajo)
// no es uno de ellos: es un estado calculado en el front para las
// compatibilidades del producto que en una conexión allowsMultipleListings
// todavía no tienen ninguna fila de publicación.
const statusLabels: Record<string, string> = {
    pending: "En cola",
    synced: "Sincronizado",
    error: "Error",
    under_review: "En revisión",
    paused: "Pausado",
    closed: "Cerrado",
}

const statusTones: Record<string, "neutral" | "success" | "warning" | "muted"> = {
    pending: "neutral",
    synced: "success",
    error: "warning",
    under_review: "warning",
    paused: "warning",
    closed: "muted",
}

function compatibilityLabel(compatibility: SyncCompatibility): string {
    const range =
        compatibility.yearEnd && compatibility.yearEnd !== compatibility.yearStart
            ? `${compatibility.yearStart}-${compatibility.yearEnd}`
            : String(compatibility.yearStart)
    const qualifiers = [compatibility.motor, compatibility.position, compatibility.side].filter(Boolean).join(" · ")
    const base = `${compatibility.brandName} ${compatibility.model} ${range}`.trim()
    return qualifiers ? `${base} (${qualifiers})` : base
}

function externalCategoryLabel(listing: SyncListing): string | undefined {
    if (!listing.externalCategoryId) {
        return undefined
    }
    return listing.externalCategoryName ? `${listing.externalCategoryName} (${listing.externalCategoryId})` : listing.externalCategoryId
}

export function ProductSyncSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "sync", productSyncSectionSchema)

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudo cargar la sincronización del producto." />
    }

    if (data.connections.length === 0) {
        return <SectionState state="empty" message="No hay conexiones de canal activas." />
    }

    return (
        <div className="flex flex-col gap-4">
            {data.connections.map((connection) => (
                <ConnectionCard key={connection.connectionId} connection={connection} />
            ))}
        </div>
    )
}

function ConnectionCard({ connection }: { connection: SyncConnection }) {
    const hasRows = connection.listings.length > 0 || connection.pending.length > 0
    const environmentLabel = connection.environment === "production" ? "Producción" : "Desarrollo"

    return (
        <SectionCard
            title={`${connection.channelName} · ${connection.connectionName}`}
            description={
                connection.allowsMultipleListings
                    ? `${environmentLabel} · publica una vez por compatibilidad`
                    : `${environmentLabel} · publicación única`
            }
        >
            {!hasRows ? (
                <SectionState state="empty" message="Este producto no está sincronizado en esta conexión." />
            ) : connection.allowsMultipleListings ? (
                <MultiListingTable connection={connection} />
            ) : (
                <SingleListing listing={connection.listings[0]} />
            )}
        </SectionCard>
    )
}

function SingleListing({ listing }: { listing: SyncListing }) {
    return (
        <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Field label="ID externo" value={listing.externalId} />
            <Field label="Categoría externa" value={externalCategoryLabel(listing)} />
            <Field label="Título publicado" value={listing.listingTitle} />
            <Field
                label="Estado"
                value={<Badge tone={statusTones[listing.status] ?? "neutral"}>{statusLabels[listing.status] ?? listing.status}</Badge>}
            />
            <Field label="Última sincronización" value={formatDate(listing.lastSyncedAt, true)} />
        </dl>
    )
}

function MultiListingTable({ connection }: { connection: SyncConnection }) {
    return (
        <Table containerClassName="overflow-x-auto">
            <TableHeader>
                <TableRow>
                    <TableHead>Compatibilidad</TableHead>
                    <TableHead>ID externo</TableHead>
                    <TableHead>Categoría externa</TableHead>
                    <TableHead>Título publicado</TableHead>
                    <TableHead>Estado</TableHead>
                    <TableHead>Última sincronización</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {connection.listings.map((listing) => (
                    <TableRow key={listing.id}>
                        <TableCell>{listing.compatibility ? compatibilityLabel(listing.compatibility) : "General"}</TableCell>
                        <TableCell>{listing.externalId ?? "—"}</TableCell>
                        <TableCell>{externalCategoryLabel(listing) ?? "—"}</TableCell>
                        <TableCell>{listing.listingTitle ?? "—"}</TableCell>
                        <TableCell>
                            <Badge tone={statusTones[listing.status] ?? "neutral"}>{statusLabels[listing.status] ?? listing.status}</Badge>
                        </TableCell>
                        <TableCell className="text-muted-foreground">{formatDate(listing.lastSyncedAt, true)}</TableCell>
                    </TableRow>
                ))}
                {connection.pending.map((compatibility) => (
                    <TableRow key={`pending-${compatibility.vehicleFitmentId}`}>
                        <TableCell>{compatibilityLabel(compatibility)}</TableCell>
                        <TableCell className="text-muted-foreground">—</TableCell>
                        <TableCell className="text-muted-foreground">—</TableCell>
                        <TableCell className="text-muted-foreground">—</TableCell>
                        <TableCell>
                            <Badge tone="warning">Pendiente</Badge>
                        </TableCell>
                        <TableCell className="text-muted-foreground">—</TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    )
}

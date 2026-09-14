import { Badge, Field, FieldError, SectionCard, SectionState } from "@/components/product-detail/ProductDetailPrimitives"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { formatDate, useProductSection, usePublishChannelListing, useResyncChannelListing } from "@/lib/product-detail"
import {
    productGeneralSchema,
    productSyncSectionSchema,
    type ListingOutcome,
    type RefreshOutcome,
    type SyncCompatibility,
    type SyncConnection,
    type SyncListing,
} from "@/lib/schemas/product-details"
import { useState } from "react"
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

// "closed" en ecom_channel_product_map.status marca una publicación que el
// marketplace cerró de forma definitiva: no se puede reabrir ni reutilizar su
// ID externo. Para esta vista una fila closed cuenta como "sin publicar" — el
// endpoint POST /api/channel-listings/publish la ignora y crea una nueva (ver
// publishOne en channel_listings/service.go) —, así que no debe ocultar el
// botón "Publicar".
const CLOSED_STATUS = "closed"

function isClosedListing(listing: SyncListing): boolean {
    return listing.status === CLOSED_STATUS
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
    // Comparte queryKey con la cabecera de ProductDetailPage y con General:
    // solo se necesita el SKU para el botón "Publicar", así que esto no repite
    // el fetch si esa sección ya lo trajo.
    const { data: general } = useProductSection(productId, "general", productGeneralSchema)

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
                <ConnectionCard
                    key={connection.connectionId}
                    productId={productId}
                    sku={general?.sku}
                    connection={connection}
                />
            ))}
        </div>
    )
}

function ConnectionCard({
    productId,
    sku,
    connection,
}: {
    productId?: string
    sku?: string
    connection: SyncConnection
}) {
    // Una fila "closed" no cuenta como publicación viva: no bloquea el botón
    // "Publicar" y se muestra aparte, con opción de volver a publicar.
    const activeListings = connection.listings.filter((listing) => !isClosedListing(listing))
    const closedListings = connection.listings.filter(isClosedListing)
    const hasActiveRows = activeListings.length > 0 || connection.pending.length > 0
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
            {!hasActiveRows ? (
                <PublishPrompt
                    productId={productId}
                    sku={sku}
                    connectionId={connection.connectionId}
                    message={
                        closedListings.length > 0
                            ? "La publicación anterior quedó cerrada (closed) en el marketplace y no se puede recuperar. Vuelve a publicar para crear una nueva."
                            : "Este producto no está sincronizado en esta conexión."
                    }
                    buttonLabel={closedListings.length > 0 ? "Volver a publicar" : "Publicar en esta conexión"}
                />
            ) : (
                <div className="flex flex-col gap-4">
                    {connection.allowsMultipleListings ? (
                        <MultiListingTable listings={activeListings} pending={connection.pending} />
                    ) : (
                        <SingleListing listing={activeListings[0]} />
                    )}
                    <ResyncButton productId={productId} connectionId={connection.connectionId} />
                    {closedListings.length > 0 ? (
                        <ClosedListingsNotice
                            productId={productId}
                            sku={sku}
                            connectionId={connection.connectionId}
                            listings={closedListings}
                        />
                    ) : null}
                </div>
            )}
        </SectionCard>
    )
}

/** Estado vacío de una conexión: mensaje + botón para crear la primera publicación. */
function PublishPrompt({
    productId,
    sku,
    connectionId,
    message,
    buttonLabel,
}: {
    productId?: string
    sku?: string
    connectionId: number
    message: string
    buttonLabel: string
}) {
    return (
        <div className="flex flex-col items-center gap-3 py-6 text-center">
            <p className="text-sm text-muted-foreground">{message}</p>
            <PublishButton productId={productId} sku={sku} connectionId={connectionId} label={buttonLabel} />
        </div>
    )
}

/**
 * Aviso para las publicaciones que el marketplace cerró (status "closed"): ya
 * no son recuperables, así que se listan de forma tenue y se ofrece volver a
 * publicar (el endpoint ignora las filas closed y crea unas nuevas).
 */
function ClosedListingsNotice({
    productId,
    sku,
    connectionId,
    listings,
}: {
    productId?: string
    sku?: string
    connectionId: number
    listings: SyncListing[]
}) {
    return (
        <div className="flex flex-col items-start gap-2 rounded-lg border border-dashed border-border p-3">
            <p className="text-xs text-muted-foreground">
                {listings.length === 1
                    ? "Una publicación quedó cerrada en el marketplace y no se puede recuperar."
                    : `${listings.length} publicaciones quedaron cerradas en el marketplace y no se pueden recuperar.`}{" "}
                Vuelve a publicar para crear una nueva.
            </p>
            <ul className="flex flex-col gap-1 text-xs text-muted-foreground">
                {listings.map((listing) => (
                    <li key={listing.id} className="flex flex-wrap items-center gap-2">
                        <Badge tone="muted">Cerrado</Badge>
                        <span>{listing.compatibility ? compatibilityLabel(listing.compatibility) : "General"}</span>
                        {listing.externalId ? <span>· {listing.externalId}</span> : null}
                    </li>
                ))}
            </ul>
            <PublishButton
                productId={productId}
                sku={sku}
                connectionId={connectionId}
                label="Volver a publicar"
                variant="outline"
            />
        </div>
    )
}

/**
 * Botón que dispara POST /api/channel-listings/publish para (conexión, SKU) y
 * muestra debajo el resultado por publicación o el error. Compartido por el
 * estado vacío de la conexión y el aviso de publicaciones cerradas.
 */
function PublishButton({
    productId,
    sku,
    connectionId,
    label,
    variant = "default",
}: {
    productId?: string
    sku?: string
    connectionId: number
    label: string
    variant?: "default" | "outline"
}) {
    const mutation = usePublishChannelListing(productId)
    const [outcomes, setOutcomes] = useState<ListingOutcome[] | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    function publish() {
        if (!sku) {
            return
        }
        setOutcomes(null)
        setErrorMessage(null)
        mutation.mutate(
            { connectionId, sku },
            {
                onSuccess: (result) => setOutcomes(result.results),
                onError: (error) => setErrorMessage(getServerErrorMessage(error, "No se pudo publicar el producto.")),
            },
        )
    }

    return (
        <>
            <Button size="sm" variant={variant} onClick={publish} disabled={mutation.isPending || !sku}>
                {mutation.isPending ? "Publicando…" : label}
            </Button>
            {outcomes ? <PublishOutcomes outcomes={outcomes} /> : null}
            <FieldError message={errorMessage} />
        </>
    )
}

/**
 * Botón que dispara POST /api/channel-listings/resync para (conexión,
 * producto) — a diferencia de PublishButton, no necesita el sku: el resync
 * siempre apunta a las publicaciones que ya existen para este producto en
 * esta conexión, nunca crea una nueva. Empuja todos los campos que el canal
 * permite modificar (atributos, descripción, imágenes; también
 * título/categoría en Odoo), no solo precio/stock como el worker
 * automático.
 */
function ResyncButton({ productId, connectionId }: { productId?: string; connectionId: number }) {
    const mutation = useResyncChannelListing(productId)
    const [outcomes, setOutcomes] = useState<RefreshOutcome[] | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    function resync() {
        setOutcomes(null)
        setErrorMessage(null)
        mutation.mutate(
            { connectionId },
            {
                onSuccess: (result) => setOutcomes(result.results),
                onError: (error) => setErrorMessage(getServerErrorMessage(error, "No se pudo resincronizar el producto.")),
            },
        )
    }

    return (
        <div className="flex flex-col items-start gap-2">
            <Button size="sm" variant="outline" onClick={resync} disabled={mutation.isPending || !productId}>
                {mutation.isPending ? "Resincronizando…" : "Resincronizar"}
            </Button>
            {outcomes ? <ResyncOutcomes outcomes={outcomes} /> : null}
            <FieldError message={errorMessage} />
        </div>
    )
}

function ResyncOutcomes({ outcomes }: { outcomes: RefreshOutcome[] }) {
    return (
        <ul className="flex w-full max-w-md flex-col gap-1 text-left text-xs">
            {outcomes.map((outcome, index) => (
                <li key={index} className={outcome.error ? "text-destructive" : "text-muted-foreground"}>
                    {outcome.error
                        ? `Error: ${outcome.error}`
                        : outcome.updated
                          ? `Resincronizado${outcome.externalId ? ` (${outcome.externalId})` : ""}.`
                          : "Sin cambios (publicación pausada, cerrada, o sin id externo todavía)."}
                </li>
            ))}
        </ul>
    )
}

function PublishOutcomes({ outcomes }: { outcomes: ListingOutcome[] }) {
    return (
        <ul className="flex w-full max-w-md flex-col gap-1 text-left text-xs">
            {outcomes.map((outcome, index) => (
                <li key={index} className={outcome.error ? "text-destructive" : "text-muted-foreground"}>
                    {outcome.error
                        ? `Error: ${outcome.error}`
                        : outcome.skipped
                          ? "Ya existía una publicación para este producto en esta conexión."
                          : `Publicado${outcome.title ? ` como «${outcome.title}»` : ""}.`}
                    {outcome.missingRequiredAttributes.length > 0
                        ? ` Faltan atributos requeridos: ${outcome.missingRequiredAttributes.join(", ")}.`
                        : ""}
                </li>
            ))}
        </ul>
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

function MultiListingTable({ listings, pending }: { listings: SyncListing[]; pending: SyncCompatibility[] }) {
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
                {listings.map((listing) => (
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
                {pending.map((compatibility) => (
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

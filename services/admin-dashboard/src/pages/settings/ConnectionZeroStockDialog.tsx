import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { apiClient } from "@/lib/api"
import { channelConnectionSchema, type ChannelConnection } from "@/lib/schemas/channel-connections"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "react"
import { z } from "zod"

interface ConnectionZeroStockDialogProps {
    connection: ChannelConnection | null
    open: boolean
    onOpenChange: (open: boolean) => void
}

// publishZeroStockResponseSchema mirrors channel_listings.CreateListingsResult:
// one outcome per SKU the batch tried to publish, so the dialog can show the
// counts (ok / error) after the user confirms. Fields outside of the ones
// used here (compatibilities etc.) are ignored.
const publishZeroStockResponseSchema = z.object({
    results: z.array(
        z.object({
            sku: z.string(),
            success: z.boolean().optional().default(false),
            externalId: z.string().optional().default(""),
            error: z.string().optional().default(""),
        }),
    ),
})

type PublishResult = z.infer<typeof publishZeroStockResponseSchema>

// ConnectionZeroStockDialog is the single place a user turns on the
// sync_zero_stock_as_paused flag per connection and manually publishes the
// backlog of elegible SKUs that already have everything (cover image, price,
// brand, category) but zero stock. The flag toggle is auto-saved on change
// so the user never leaves the dialog in a "dirty" state; the batch publish
// is behind an explicit confirmation and reports per-SKU outcomes.
export function ConnectionZeroStockDialog({ connection, open, onOpenChange }: ConnectionZeroStockDialogProps) {
    const queryClient = useQueryClient()
    const [enabled, setEnabled] = useState(connection?.syncZeroStockAsPaused ?? false)
    const [publishResult, setPublishResult] = useState<PublishResult | null>(null)

    useEffect(() => {
        setEnabled(connection?.syncZeroStockAsPaused ?? false)
        setPublishResult(null)
    }, [connection?.id, connection?.syncZeroStockAsPaused])

    const saveFlag = useMutation({
        // Reuses the full Upsert endpoint rather than inventing a PATCH path:
        // Update*ChannelConnection takes the whole input today, so this just
        // re-sends every other field unchanged alongside the one we're
        // actually touching.
        mutationFn: async (nextEnabled: boolean) => {
            if (!connection) return
            const response = await apiClient.put(`/api/channel-connections/${connection.id}`, {
                channelId: connection.channelId,
                name: connection.name,
                status: connection.status,
                environment: connection.environment,
                currencyId: connection.currencyId,
                allowsMultipleListings: connection.allowsMultipleListings,
                syncZeroStockAsPaused: nextEnabled,
            })
            return channelConnectionSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["channel-connections"] })
        },
    })

    const publishEligibles = useMutation({
        mutationFn: async (limit: number) => {
            if (!connection) throw new Error("no connection")
            const response = await apiClient.post("/api/channel-listings/publish-zero-stock-eligibles", {
                connectionId: connection.id,
                limit,
            })
            return publishZeroStockResponseSchema.parse(response.data)
        },
        onSuccess: (data) => {
            setPublishResult(data)
            queryClient.invalidateQueries({ queryKey: ["channel-connections"] })
        },
    })

    if (!connection) return null

    const successCount = publishResult?.results.filter((r) => r.success).length ?? 0
    const errorCount = publishResult?.results.filter((r) => !r.success).length ?? 0

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>Suspender publicaciones sin stock</DialogTitle>
                    <DialogDescription>
                        Configura esta conexión ({connection.name}) para publicar como
                        suspendido lo que ya está listo pero no tiene inventario, y
                        reactivarlo automáticamente cuando el stock regrese.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex items-start gap-3 py-3">
                    <input
                        id={`sync-zero-stock-${connection.id}`}
                        type="checkbox"
                        className="mt-1 size-4"
                        checked={enabled}
                        disabled={saveFlag.isPending}
                        onChange={(e) => {
                            const next = e.target.checked
                            setEnabled(next)
                            saveFlag.mutate(next)
                        }}
                    />
                    <label htmlFor={`sync-zero-stock-${connection.id}`} className="flex-1 cursor-pointer">
                        <div className="text-sm font-medium">
                            Publicar artículos sin stock como suspendidos
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                            Aplica a los productos con foto, precio, marca y categoría.
                            En MercadoLibre quedan como <em>paused</em>; en Odoo con
                            <em> no se puede comprar</em> (sale_ok=false). Cuando el
                            stock vuelva, los que el sistema haya pausado se reactivan
                            solos — los que un humano haya pausado a mano no.
                        </p>
                    </label>
                </div>

                {saveFlag.isError ? (
                    <p className="text-xs text-destructive">
                        No se pudo guardar la configuración:{" "}
                        {saveFlag.error instanceof Error ? saveFlag.error.message : "error desconocido"}
                    </p>
                ) : null}

                <div className="rounded-lg border border-border p-3">
                    <p className="text-sm font-medium">Publicar elegibles sin stock ahora</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                        Busca productos activos con foto, precio y datos completos que
                        hoy están en 0 piezas y aún no se han publicado en esta
                        conexión. Los sube suspendidos (si el toggle de arriba está
                        prendido) o activos con qty=0 (si no lo está, decisión que
                        tomas en el toggle de arriba).
                    </p>

                    <div className="mt-3 flex flex-wrap gap-2">
                        <Button
                            variant="outline"
                            disabled={publishEligibles.isPending}
                            onClick={() => publishEligibles.mutate(10)}
                            title="Procesa a lo más 10 SKUs — ideal para una prueba rápida antes de correr todo el backlog."
                        >
                            {publishEligibles.isPending ? "Publicando..." : "Probar con 10"}
                        </Button>
                        <Button
                            variant="default"
                            disabled={publishEligibles.isPending}
                            onClick={() => {
                                if (confirm("Se publicarán todos los elegibles sin stock. ¿Continuar?")) {
                                    publishEligibles.mutate(0)
                                }
                            }}
                        >
                            {publishEligibles.isPending ? "Publicando..." : "Publicar todos"}
                        </Button>
                    </div>

                    {publishResult ? (
                        <div className="mt-3 text-xs">
                            <p className="font-medium">
                                Resultado: {successCount} publicados, {errorCount} con error
                                {publishResult.results.length === 0 ? " (nada elegible)" : ""}
                            </p>
                            {errorCount > 0 ? (
                                <ul className="mt-2 max-h-32 list-disc overflow-auto pl-4 text-muted-foreground">
                                    {publishResult.results
                                        .filter((r) => !r.success)
                                        .slice(0, 50)
                                        .map((r) => (
                                            <li key={r.sku}>
                                                <code>{r.sku}</code>: {r.error}
                                            </li>
                                        ))}
                                </ul>
                            ) : null}
                        </div>
                    ) : null}

                    {publishEligibles.isError ? (
                        <p className="mt-2 text-xs text-destructive">
                            Falló la publicación:{" "}
                            {publishEligibles.error instanceof Error
                                ? publishEligibles.error.message
                                : "error desconocido"}
                        </p>
                    ) : null}
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cerrar
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

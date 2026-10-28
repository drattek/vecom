import { Button } from "@/components/ui/button"
import { getServerErrorMessage } from "@/lib/api"
import { useEnqueueConnectionResync } from "@/lib/channel-listings"
import { Loader2Icon, RefreshCwIcon } from "lucide-react"
import { useState } from "react"

/**
 * Botón de la vista de una conexión (MercadoLibre y Odoo): manda a la cola a
 * todos los productos publicados en ella para hacerles el mismo "Resincronizar"
 * que el del detalle del producto. Precio y stock se envían aunque no hayan
 * cambiado (útil tras modificar una fórmula de precios). El envío lo hace el
 * worker en segundo plano; aquí solo se confirma el encolado.
 */
export function EnqueueResyncButton({ connectionId }: { connectionId: number }) {
    const mutation = useEnqueueConnectionResync()
    const [message, setMessage] = useState<string | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    function enqueue() {
        if (
            !window.confirm(
                "Se encolarán todos los productos publicados en esta conexión para resincronizarlos (precio y stock se envían aunque no hayan cambiado). ¿Continuar?",
            )
        ) {
            return
        }

        setMessage(null)
        setErrorMessage(null)
        mutation.mutate(
            { connectionId },
            {
                onSuccess: ({ enqueued, requeued }) => {
                    const total = enqueued + requeued
                    setMessage(
                        total === 0
                            ? "No hay productos nuevos por encolar: los pendientes ya están en la cola."
                            : `${total} producto(s) encolados (${enqueued} nuevos, ${requeued} reencolados). Se procesan en segundo plano.`,
                    )
                },
                onError: (error) => setErrorMessage(getServerErrorMessage(error, "No se pudo encolar la resincronización.")),
            },
        )
    }

    return (
        <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" onClick={enqueue} disabled={mutation.isPending}>
                {mutation.isPending ? <Loader2Icon className="animate-spin" /> : <RefreshCwIcon />}
                {mutation.isPending ? "Encolando…" : "Resincronizar"}
            </Button>
            {message ? <span className="text-sm text-muted-foreground">{message}</span> : null}
            {errorMessage ? <span className="text-sm text-destructive">{errorMessage}</span> : null}
        </div>
    )
}

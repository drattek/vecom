import { Button } from "@/components/ui/button"
import { getServerErrorMessage } from "@/lib/api"
import { useEnqueueConnectionCompatibilities } from "@/lib/channel-listings"
import { ListChecksIcon, Loader2Icon } from "lucide-react"
import { useState } from "react"

/**
 * Botón de la vista de una conexión: manda a la cola a todos los productos
 * publicados en ella para que sincronicen sus compatibilidades (maquinaria y
 * vehículos), tengan o no, sin tocar precio, stock, atributos ni imágenes. El
 * envío lo hace el worker en segundo plano; aquí solo se confirma el encolado.
 */
export function EnqueueCompatibilitiesButton({ connectionId }: { connectionId: number }) {
    const mutation = useEnqueueConnectionCompatibilities()
    const [message, setMessage] = useState<string | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    function enqueue() {
        if (!window.confirm("Se encolarán todos los productos publicados en esta conexión para sincronizar sus compatibilidades. ¿Continuar?")) {
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
                onError: (error) =>
                    setErrorMessage(getServerErrorMessage(error, "No se pudieron encolar las compatibilidades.")),
            },
        )
    }

    return (
        <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" onClick={enqueue} disabled={mutation.isPending}>
                {mutation.isPending ? <Loader2Icon className="animate-spin" /> : <ListChecksIcon />}
                {mutation.isPending ? "Encolando…" : "Sincronizar compatibilidades"}
            </Button>
            {message ? <span className="text-sm text-muted-foreground">{message}</span> : null}
            {errorMessage ? <span className="text-sm text-destructive">{errorMessage}</span> : null}
        </div>
    )
}

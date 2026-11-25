import { useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import type { ChannelConnection } from "@/lib/schemas/channel-connections"

const authorizationSchema = z.object({
    authorizationUrl: z.url({ protocol: /^https$/, hostname: /(^|\.)mercadolibre\.com(\.[a-z]{2})?$/ }),
})

interface MercadoLibreLinkDialogProps {
    connection: ChannelConnection
    onClose: () => void
}

/**
 * Vincula la conexión con una cuenta de MercadoLibre. Se monta solo mientras
 * está abierto, así que cada apertura empieza sin estado previo.
 */
export function MercadoLibreLinkDialog({ connection, onClose }: MercadoLibreLinkDialogProps) {
    const [allowAccountChange, setAllowAccountChange] = useState(false)

    const start = useMutation({
        mutationFn: async () => {
            const response = await apiClient.post(
                `/api/channel-connections/${connection.id}/mercadolibre/authorize`,
                { allowAccountChange },
            )
            return authorizationSchema.parse(response.data).authorizationUrl
        },
        onSuccess: (authorizationUrl) => {
            window.location.assign(authorizationUrl)
        },
    })

    // isSuccess: el navegador ya está saliendo hacia MercadoLibre.
    const leaving = start.isPending || start.isSuccess

    return (
        <Dialog open onOpenChange={(open) => (open ? undefined : onClose())}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>Vincular cuenta de MercadoLibre</DialogTitle>
                    <DialogDescription>
                        Conexión «{connection.name}». Irás a MercadoLibre para autorizar la aplicación y volverás a esta
                        pantalla.
                    </DialogDescription>
                </DialogHeader>

                <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                    <p className="font-medium">MercadoLibre usará la sesión que tenga abierta tu navegador.</p>
                    <p className="mt-1">
                        Inicia sesión con la <strong>cuenta administradora</strong> que corresponde a esta conexión; un
                        operador o colaborador no puede autorizar. Si usas varias cuentas, haz este proceso en una
                        ventana privada.
                    </p>
                </div>

                <label className="flex cursor-pointer items-start gap-2 text-sm">
                    <input
                        type="checkbox"
                        className="mt-1 size-4"
                        checked={allowAccountChange}
                        disabled={leaving}
                        onChange={(event) => setAllowAccountChange(event.target.checked)}
                    />
                    <span>
                        Quiero cambiar la cuenta de esta conexión
                        <span className="block text-xs text-muted-foreground">
                            Márcalo solo si es a propósito. Las publicaciones y ventas de esta conexión pasarán a
                            consultarse con la cuenta nueva. Sin marcarlo, una cuenta distinta se rechaza.
                        </span>
                    </span>
                </label>

                {start.isError ? (
                    <p className="text-sm text-destructive" role="alert">
                        {getServerErrorMessage(start.error, "No se pudo iniciar la vinculación.")}
                    </p>
                ) : null}

                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={leaving}>
                        Cancelar
                    </Button>
                    <Button onClick={() => start.mutate()} disabled={leaving}>
                        {leaving ? "Abriendo MercadoLibre..." : "Continuar a MercadoLibre"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

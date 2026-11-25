import { CircleCheckIcon, TriangleAlertIcon, XIcon } from "lucide-react"
import { useQuery } from "@tanstack/react-query"
import { useSearchParams } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { apiClient } from "@/lib/api"
import { channelConnectionSchema } from "@/lib/schemas/channel-connections"

// Motivos que envía el core al regresar de MercadoLibre (/meli_callback).
const REASONS: Record<string, string> = {
    denied: "La autorización se canceló o MercadoLibre no la otorgó.",
    invalid_state: "El enlace de vinculación caducó o ya se usó. Inicia la vinculación otra vez.",
    account_mismatch:
        "Autorizaste una cuenta distinta a la que esta conexión tiene vinculada. Inicia sesión con la cuenta correcta o, si quieres cambiarla, marca la opción al vincular.",
    account_in_use:
        "La cuenta autorizada ya está vinculada a otra conexión. Revisa con qué cuenta inició sesión el navegador o, si es a propósito, marca la opción de cambiar la cuenta.",
    missing_settings: "Faltan ajustes en la conexión (app_id, client_secret o redirect_url).",
    connection: "La conexión ya no existe o no es de MercadoLibre.",
    exchange:
        "MercadoLibre no entregó los tokens. Revisa el app_id, el client_secret y que redirect_url sea idéntico al registrado en la aplicación.",
    internal: "No se pudo completar la vinculación. Revisa el registro del servidor.",
}

const PARAMS = ["meliAuth", "reason", "account", "connectionId"]

/** Aviso con el resultado de la vinculación, leído de la URL de regreso. */
export function MercadoLibreLinkResult() {
    const [params, setParams] = useSearchParams()
    const outcome = params.get("meliAuth")
    const connectionId = Number(params.get("connectionId")) || null

    const connection = useQuery({
        queryKey: ["channel-connections", "one", connectionId],
        enabled: outcome !== null && connectionId !== null,
        retry: false,
        queryFn: async () => {
            const response = await apiClient.get(`/api/channel-connections/${connectionId}`)
            return channelConnectionSchema.parse(response.data)
        },
    })

    if (outcome !== "success" && outcome !== "error") return null

    const success = outcome === "success"
    const name = connection.data ? `«${connection.data.name}»` : connectionId ? `#${connectionId}` : ""
    const account = params.get("account")

    const dismiss = () => {
        setParams(
            (current) => {
                const next = new URLSearchParams(current)
                PARAMS.forEach((key) => next.delete(key))
                return next
            },
            { replace: true },
        )
    }

    return (
        <div
            role={success ? "status" : "alert"}
            className={`mt-3 flex shrink-0 items-start gap-2 rounded-lg border p-3 text-sm ${
                success ? "border-emerald-500/40 bg-emerald-500/10" : "border-destructive/40 bg-destructive/10"
            }`}
        >
            {success ? (
                <CircleCheckIcon className="mt-0.5 size-4 shrink-0" />
            ) : (
                <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
            )}
            <p className="min-w-0 flex-1 break-words">
                {success
                    ? `La conexión ${name} quedó vinculada${account ? ` a la cuenta ${account}` : ""}.`
                    : `No se vinculó la conexión ${name}. ${REASONS[params.get("reason") ?? ""] ?? REASONS.internal} No se modificaron sus tokens.`}
            </p>
            <Button variant="ghost" size="icon-sm" aria-label="Cerrar aviso" onClick={dismiss}>
                <XIcon />
            </Button>
        </div>
    )
}

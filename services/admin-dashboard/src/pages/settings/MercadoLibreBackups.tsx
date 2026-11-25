import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import { formatDateTime } from "@/lib/dashboard"
import { backupsKey, type useRestoreBackup } from "./useRestoreBackup"

// Respaldos automáticos de la configuración de MercadoLibre de una conexión.

const backupSchema = z.object({
    id: z.number(),
    reason: z.string(),
    appId: z.string().nullable(),
    createdAt: z.string(),
    createdById: z.number(),
    createdBy: z.string().nullable(),
})

const backupsSchema = z.object({ backups: z.array(backupSchema) })

type Backup = z.infer<typeof backupSchema>

// Cada respaldo es lo que había justo antes de la acción.
const REASONS: Record<string, string> = {
    app_update: "Antes de editar la aplicación",
    account_link: "Antes de vincular una cuenta",
    restore: "Antes de restaurar",
}

interface MercadoLibreBackupsProps {
    connectionId: number
    restore: ReturnType<typeof useRestoreBackup>
    disabled: boolean
}

export function MercadoLibreBackups({ connectionId, restore, disabled }: MercadoLibreBackupsProps) {
    const [confirming, setConfirming] = useState<number | null>(null)

    const backups = useQuery({
        queryKey: backupsKey(connectionId),
        retry: false,
        refetchOnMount: "always",
        queryFn: async () => {
            const response = await apiClient.get(`/api/channel-connections/${connectionId}/mercadolibre/backups`)
            return backupsSchema.parse(response.data).backups
        },
    })

    const confirmRestore = (backup: Backup) => {
        setConfirming(null)
        restore.mutate(backup.id)
    }

    return (
        <section className="border-t pt-3">
            <h3 className="text-sm font-medium">Configuraciones anteriores</h3>
            <p className="mt-1 text-xs text-muted-foreground">
                Se respaldan solas antes de editar, vincular o restaurar. Se muestran las 5 más recientes.
            </p>

            {restore.isSuccess ? (
                restore.data.accessVerified ? (
                    <p className="mt-2 rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-2 text-sm" role="status">
                        Configuración restaurada (App ID {restore.data.appId}). MercadoLibre aceptó los tokens.
                    </p>
                ) : (
                    <p className="mt-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-2 text-sm" role="alert">
                        Configuración restaurada (App ID {restore.data.appId}), pero no se pudo comprobar el acceso: los
                        tokens pueden haber dejado de servir. Vuelve a vincular la cuenta; el redirect_url de esa
                        aplicación debe apuntar a este sistema.
                    </p>
                )
            ) : null}
            {restore.isError ? (
                <p className="mt-2 text-sm text-destructive" role="alert">
                    {getServerErrorMessage(restore.error, "No se pudo restaurar el respaldo.")}
                </p>
            ) : null}

            {backups.isLoading ? <p className="mt-2 text-sm text-muted-foreground">Cargando respaldos...</p> : null}
            {backups.isError ? (
                <p className="mt-2 text-sm text-destructive">
                    {getServerErrorMessage(backups.error, "No se pudieron consultar los respaldos.")}
                </p>
            ) : null}
            {backups.data?.length === 0 ? (
                <p className="mt-2 text-sm text-muted-foreground">Todavía no hay respaldos automáticos.</p>
            ) : null}

            <ul className="mt-2 space-y-2">
                {backups.data?.map((backup) => (
                    <li key={backup.id} className="rounded-lg border border-border p-2 text-sm">
                        <div className="flex items-start justify-between gap-2">
                            <div className="min-w-0">
                                <p>{formatDateTime(backup.createdAt)}</p>
                                <p className="text-xs text-muted-foreground">
                                    {REASONS[backup.reason] ?? backup.reason} · App ID {backup.appId ?? "sin dato"} ·{" "}
                                    {backup.createdBy ?? `usuario #${backup.createdById}`}
                                </p>
                            </div>
                            {confirming !== backup.id ? (
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    disabled={disabled || restore.isPending}
                                    onClick={() => setConfirming(backup.id)}
                                >
                                    {restore.isPending && restore.variables === backup.id ? "Restaurando..." : "Restaurar"}
                                </Button>
                            ) : null}
                        </div>
                        {confirming === backup.id ? (
                            <div className="mt-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-2">
                                <p>
                                    Se reemplazarán la aplicación y los tokens de esta conexión por los de este respaldo. La
                                    configuración actual se respalda antes. Los tokens anteriores podrían ya no servir.
                                </p>
                                <div className="mt-2 flex justify-end gap-2">
                                    <Button type="button" variant="outline" size="sm" onClick={() => setConfirming(null)}>
                                        Cancelar
                                    </Button>
                                    <Button type="button" size="sm" disabled={disabled} onClick={() => confirmRestore(backup)}>
                                        Sí, restaurar
                                    </Button>
                                </div>
                            </div>
                        ) : null}
                    </li>
                ))}
            </ul>
        </section>
    )
}

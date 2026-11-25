import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useForm, useWatch } from "react-hook-form"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import type { ChannelConnection } from "@/lib/schemas/channel-connections"

const appSettingsSchema = z.object({
    appId: z.string(),
    redirectUrl: z.string(),
    hasClientSecret: z.boolean(),
    linkedAccountId: z.number().nullable(),
})

const savedAppSchema = appSettingsSchema.extend({ appChanged: z.boolean() })

type AppSettings = z.infer<typeof appSettingsSchema>
type SavedApp = z.infer<typeof savedAppSchema>
type FormValues = { appId: string; clientSecret: string; redirectUrl: string }

// Mismas reglas que el core; él es quien decide.
const appIdRule = z.string().trim().regex(/^[0-9]{5,20}$/, "Es el número de la aplicación (solo dígitos).")
const redirectRule = z
    .url({ protocol: /^https$/, error: "Debe ser una URL https." })
    .refine((value) => {
        const url = new URL(value)
        return url.pathname === "/meli_callback" && !url.search && !url.hash
    }, "Debe terminar en /meli_callback, sin parámetros.")

function check(rule: z.ZodType, value: string): true | string {
    const result = rule.safeParse(value.trim())
    return result.success ? true : result.error.issues[0].message
}

interface MercadoLibreAppDialogProps {
    connection: ChannelConnection
    onClose: () => void
    /** Continúa con la vinculación de la cuenta. */
    onLink: () => void
}

/**
 * Cambia la aplicación de MercadoLibre con la que se autentica una conexión.
 * Se monta solo mientras está abierto, así que el secreto capturado no
 * sobrevive al cierre.
 */
export function MercadoLibreAppDialog({ connection, onClose, onLink }: MercadoLibreAppDialogProps) {
    const queryClient = useQueryClient()
    const endpoint = `/api/channel-connections/${connection.id}/mercadolibre/app`
    const queryKey = ["channel-connections", "mercadolibre-app", connection.id]

    // Sin caché ni recargas automáticas: no debe pisar lo que se está editando.
    const current = useQuery({
        queryKey,
        gcTime: 0,
        retry: false,
        refetchOnWindowFocus: false,
        queryFn: async () => {
            const response = await apiClient.get(endpoint)
            return appSettingsSchema.parse(response.data)
        },
    })

    const save = useMutation({
        // gcTime 0: las variables de la petición incluyen el client_secret.
        gcTime: 0,
        mutationFn: async (values: FormValues) => {
            const response = await apiClient.put(endpoint, values)
            return savedAppSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["channel-connection-config", connection.id] })
        },
    })

    return (
        <Dialog open onOpenChange={(open) => (open || save.isPending ? undefined : onClose())}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>Editar aplicación de MercadoLibre</DialogTitle>
                    <DialogDescription>
                        Conexión «{connection.name}». La conexión, sus publicaciones vinculadas y sus ventas no cambian.
                    </DialogDescription>
                </DialogHeader>

                {save.data ? (
                    <SavedNotice saved={save.data} onClose={onClose} onLink={onLink} />
                ) : (
                    <>
                        {current.isLoading ? (
                            <p className="text-sm text-muted-foreground">Cargando datos de la aplicación...</p>
                        ) : null}
                        {current.isError ? (
                            <>
                                <p className="text-sm text-destructive" role="alert">
                                    {getServerErrorMessage(current.error, "No se pudieron consultar los datos de la aplicación.")}
                                </p>
                                <DialogFooter>
                                    <Button variant="outline" onClick={onClose}>Cerrar</Button>
                                </DialogFooter>
                            </>
                        ) : null}
                        {current.data ? (
                            <AppForm
                                current={current.data}
                                saving={save.isPending}
                                error={
                                    save.isError
                                        ? getServerErrorMessage(save.error, "No se pudieron guardar los datos de la aplicación.")
                                        : null
                                }
                                onCancel={onClose}
                                onSubmit={(values) => save.mutate(values)}
                            />
                        ) : null}
                    </>
                )}
            </DialogContent>
        </Dialog>
    )
}

function SavedNotice({ saved, onClose, onLink }: { saved: SavedApp; onClose: () => void; onLink: () => void }) {
    return (
        <>
            <div className="space-y-2 text-sm" role="status">
                <p className="font-medium">Datos guardados.</p>
                {saved.appChanged ? (
                    <p className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3">
                        <strong>Vincula la cuenta ahora.</strong> Los tokens actuales son de la aplicación anterior y ya no
                        se podrán renovar: al vencer, la publicación y las ventas de esta conexión fallarán.
                    </p>
                ) : (
                    <p>La aplicación es la misma. Vuelve a vincular solo si cambiaste el redirect_url o los permisos.</p>
                )}
                {saved.appChanged && saved.linkedAccountId === null ? (
                    <p className="text-destructive">
                        No se pudo identificar la cuenta que tenía la conexión. Al vincular, comprueba que inicias sesión
                        con la cuenta correcta.
                    </p>
                ) : null}
            </div>
            <DialogFooter>
                <Button variant="outline" onClick={onClose}>Cerrar</Button>
                <Button onClick={onLink}>Vincular cuenta ahora</Button>
            </DialogFooter>
        </>
    )
}

interface AppFormProps {
    current: AppSettings
    saving: boolean
    error: string | null
    onCancel: () => void
    onSubmit: (values: FormValues) => void
}

function AppForm({ current, saving, error, onCancel, onSubmit }: AppFormProps) {
    const suggestedRedirect = `${window.location.origin}/meli_callback`
    const {
        register,
        handleSubmit,
        setValue,
        control,
        formState: { errors, isDirty },
    } = useForm<FormValues>({
        defaultValues: { appId: current.appId, clientSecret: "", redirectUrl: current.redirectUrl },
    })
    const appId = useWatch({ control, name: "appId" })
    const redirectUrl = useWatch({ control, name: "redirectUrl" })
    const appChanged = appId.trim() !== current.appId
    const secretRequired = appChanged || !current.hasClientSecret

    return (
        <form
            className="space-y-3"
            noValidate
            onSubmit={handleSubmit((values) =>
                onSubmit({
                    appId: values.appId.trim(),
                    clientSecret: values.clientSecret.trim(),
                    redirectUrl: values.redirectUrl.trim(),
                }),
            )}
        >
            <div className="space-y-1">
                <Label htmlFor="meli-app-id">App ID</Label>
                <Input
                    id="meli-app-id"
                    inputMode="numeric"
                    autoComplete="off"
                    disabled={saving}
                    aria-invalid={errors.appId ? true : undefined}
                    {...register("appId", { validate: (value) => check(appIdRule, value) })}
                />
                {errors.appId ? <p className="text-xs text-destructive">{errors.appId.message}</p> : null}
            </div>

            <div className="space-y-1">
                <Label htmlFor="meli-client-secret">Client secret</Label>
                <Input
                    id="meli-client-secret"
                    type="password"
                    // new-password evita que el navegador rellene aquí la contraseña guardada.
                    autoComplete="new-password"
                    disabled={saving}
                    placeholder={secretRequired ? "Obligatorio" : "Déjalo vacío para conservar el actual"}
                    aria-invalid={errors.clientSecret ? true : undefined}
                    {...register("clientSecret", {
                        validate: (value) =>
                            !secretRequired || value.trim() !== "" || "Captura el client secret de la aplicación.",
                    })}
                />
                {errors.clientSecret ? (
                    <p className="text-xs text-destructive">{errors.clientSecret.message}</p>
                ) : (
                    <p className="text-xs text-muted-foreground">
                        Se guarda cifrado y no se vuelve a mostrar.
                    </p>
                )}
            </div>

            <div className="space-y-1">
                <Label htmlFor="meli-redirect-url">Redirect URL</Label>
                <Input
                    id="meli-redirect-url"
                    type="url"
                    autoComplete="off"
                    disabled={saving}
                    aria-invalid={errors.redirectUrl ? true : undefined}
                    {...register("redirectUrl", { validate: (value) => check(redirectRule, value) })}
                />
                {errors.redirectUrl ? <p className="text-xs text-destructive">{errors.redirectUrl.message}</p> : null}
                <p className="text-xs text-muted-foreground">
                    Debe ser idéntico al registrado en la aplicación.
                </p>
                {redirectUrl.trim() !== suggestedRedirect && suggestedRedirect.startsWith("https://") ? (
                    <Button
                        type="button"
                        variant="link"
                        size="sm"
                        className="h-auto px-0 text-xs break-all whitespace-normal"
                        disabled={saving}
                        onClick={() => setValue("redirectUrl", suggestedRedirect, { shouldDirty: true, shouldValidate: true })}
                    >
                        Usar {suggestedRedirect}
                    </Button>
                ) : null}
            </div>

            {appChanged ? (
                <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                    <p className="font-medium">Vas a cambiar de aplicación.</p>
                    <p className="mt-1">
                        Antes de guardar: respalda los ajustes de la conexión y comprueba que la aplicación nueva tenga sus
                        permisos, este redirect URL y PKCE desactivado. Después tendrás que vincular la cuenta enseguida.
                    </p>
                </div>
            ) : null}

            {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}

            <DialogFooter>
                <Button type="button" variant="outline" onClick={onCancel} disabled={saving}>
                    Cancelar
                </Button>
                <Button type="submit" disabled={saving || !isDirty}>
                    {saving ? "Guardando..." : "Guardar"}
                </Button>
            </DialogFooter>
        </form>
    )
}

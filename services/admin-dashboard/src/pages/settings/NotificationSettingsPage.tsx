import { useState, type FormEvent } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getServerErrorMessage } from "@/lib/api"
import { useNotificationSettings, useSaveNotificationSettings } from "@/lib/notifications"
import type { NotificationSettings } from "@/lib/schemas/notifications"

export function NotificationSettingsPage() {
    const { data, isLoading, isError, error } = useNotificationSettings()

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-y-auto p-4">
            <h2 className="text-xl font-semibold text-foreground">Notificaciones por Telegram</h2>
            <p className="mt-2 max-w-xl text-sm text-muted-foreground">
                Configura el bot que envía los avisos. Cada usuario vincula su Telegram y elige qué recibir en Mi perfil.
            </p>

            {isLoading ? <p className="mt-4 text-sm text-muted-foreground">Cargando...</p> : null}
            {isError ? (
                <p className="mt-4 text-sm text-destructive">
                    {getServerErrorMessage(error, "No se pudo obtener la configuración.")}
                </p>
            ) : null}
            {data ? <SettingsForm settings={data} /> : null}
        </section>
    )
}

function SettingsForm({ settings }: { settings: NotificationSettings }) {
    const save = useSaveNotificationSettings()
    const [botToken, setBotToken] = useState("")
    const [isEnabled, setIsEnabled] = useState(settings.isEnabled)
    const [dailySummaryTime, setDailySummaryTime] = useState(settings.dailySummaryTime)
    const [erpSilenceHours, setErpSilenceHours] = useState(String(settings.erpSilenceHours))
    const [connectionCheckMinutes, setConnectionCheckMinutes] = useState(String(settings.connectionCheckMinutes))
    const [notice, setNotice] = useState<{ tone: "ok" | "error"; text: string } | null>(null)

    async function handleSubmit(event: FormEvent) {
        event.preventDefault()
        setNotice(null)
        try {
            await save.mutateAsync({
                botToken: botToken.trim(),
                isEnabled,
                dailySummaryTime,
                erpSilenceHours: Number(erpSilenceHours),
                connectionCheckMinutes: Number(connectionCheckMinutes),
            })
            setBotToken("")
            setNotice({ tone: "ok", text: "Configuración guardada." })
        } catch (submitError) {
            setNotice({ tone: "error", text: getServerErrorMessage(submitError, "No se pudo guardar la configuración.") })
        }
    }

    return (
        <form className="mt-6 max-w-xl space-y-5" onSubmit={handleSubmit}>
            <div className="space-y-2">
                <Label htmlFor="bot-token">Token del bot</Label>
                <Input
                    id="bot-token"
                    type="password"
                    autoComplete="new-password"
                    value={botToken}
                    placeholder={settings.hasToken ? "Guardado (escribe para reemplazarlo)" : "Pega el token de @BotFather"}
                    onChange={(event) => setBotToken(event.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                    {settings.botUsername ? `Bot actual: @${settings.botUsername}. ` : ""}
                    Se guarda cifrado y no se vuelve a mostrar. Usa un bot distinto en cada entorno. Si cambias de bot,
                    todos deben vincular su cuenta otra vez.
                </p>
            </div>

            <label className="flex items-center gap-2 text-sm font-medium text-foreground">
                <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={isEnabled}
                    onChange={(event) => setIsEnabled(event.target.checked)}
                />
                Activar notificaciones
            </label>

            <div className="space-y-2">
                <Label htmlFor="summary-time">Hora del resumen diario ({settings.timezone})</Label>
                <Input
                    id="summary-time"
                    type="time"
                    required
                    className="w-40"
                    value={dailySummaryTime}
                    onChange={(event) => setDailySummaryTime(event.target.value)}
                />
            </div>

            <div className="space-y-2">
                <Label htmlFor="erp-silence">Horas sin sincronización del ERP antes de avisar</Label>
                <Input
                    id="erp-silence"
                    type="number"
                    required
                    min={1}
                    max={240}
                    className="w-40"
                    value={erpSilenceHours}
                    onChange={(event) => setErpSilenceHours(event.target.value)}
                />
            </div>

            <div className="space-y-2">
                <Label htmlFor="connection-check">Minutos entre comprobaciones de conexiones</Label>
                <Input
                    id="connection-check"
                    type="number"
                    required
                    min={5}
                    max={1440}
                    className="w-40"
                    value={connectionCheckMinutes}
                    onChange={(event) => setConnectionCheckMinutes(event.target.value)}
                />
            </div>

            <Button type="submit" disabled={save.isPending}>
                {save.isPending ? "Guardando..." : "Guardar"}
            </Button>

            {notice ? (
                <p
                    className={`text-sm ${notice.tone === "error" ? "text-destructive" : "text-muted-foreground"}`}
                    role="status"
                >
                    {notice.text}
                </p>
            ) : null}
        </form>
    )
}

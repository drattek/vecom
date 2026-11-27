import { useEffect, useState } from "react"
import { ExternalLinkIcon, SendIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { getServerErrorMessage } from "@/lib/api"
import {
    useMyNotifications,
    useSaveNotificationPreference,
    useSendTelegramTest,
    useSetNotificationsEnabled,
    useStartTelegramLink,
    useUnlinkTelegram,
} from "@/lib/notifications"
import type { MyNotifications, NotificationPreference, TelegramLinkRequest } from "@/lib/schemas/notifications"

const DELIVERY_LABELS: Record<string, string> = {
    instant: "Al momento",
    after_batch: "Al terminar la tanda",
    daily_summary: "Resumen diario",
    after_check: "0:00 y 12:00",
}

type Notice = { tone: "ok" | "error"; text: string }

function groupPreferences(preferences: NotificationPreference[]) {
    const groups = new Map<string, NotificationPreference[]>()
    for (const preference of preferences) {
        groups.set(preference.group, [...(groups.get(preference.group) ?? []), preference])
    }
    return [...groups.entries()]
}

function isLinked(data: MyNotifications | undefined) {
    return data?.link.status === "linked" && !data.link.needsRelink
}

export function TelegramNotificationsCard() {
    const [linkRequest, setLinkRequest] = useState<TelegramLinkRequest | null>(null)
    const [notice, setNotice] = useState<Notice | null>(null)

    const { data, isLoading, isError, error } = useMyNotifications(linkRequest !== null)
    const startLink = useStartTelegramLink()
    const unlink = useUnlinkTelegram()
    const sendTest = useSendTelegramTest()
    const setEnabled = useSetNotificationsEnabled()
    const savePreference = useSaveNotificationPreference()

    const linked = isLinked(data)
    // El enlace se oculta al completarse la vinculación.
    const pendingRequest = linked ? null : linkRequest

    // Al vencer el enlace se deja de consultar.
    useEffect(() => {
        if (!linkRequest) {
            return
        }
        const timer = window.setTimeout(
            () => setLinkRequest(null),
            linked ? 0 : Math.max(new Date(linkRequest.expiresAt).getTime() - Date.now(), 0),
        )
        return () => window.clearTimeout(timer)
    }, [linkRequest, linked])

    async function run(action: () => Promise<unknown>, success: string | null, failure: string) {
        setNotice(null)
        try {
            await action()
            if (success) {
                setNotice({ tone: "ok", text: success })
            }
        } catch (actionError) {
            setNotice({ tone: "error", text: getServerErrorMessage(actionError, failure) })
        }
    }

    function requestLink() {
        return run(
            async () => setLinkRequest(await startLink.mutateAsync()),
            null,
            "No se pudo generar el enlace.",
        )
    }

    const link = data?.link
    const hasLink = link !== undefined && link.status !== "none"
    const accountName = link?.telegramUsername ? `@${link.telegramUsername}` : link?.telegramName

    return (
        <section className="m-4 max-w-xl rounded-xl border bg-card p-6">
            <h2 className="text-xl font-semibold text-foreground">Notificaciones por Telegram</h2>
            <p className="mt-2 text-sm text-muted-foreground">
                Recibe en tu Telegram, en privado, los avisos que elijas. Incluyen todas las conexiones que correspondan.
            </p>

            {isLoading ? <p className="mt-4 text-sm text-muted-foreground">Cargando...</p> : null}
            {isError ? (
                <p className="mt-4 text-sm text-destructive">
                    {getServerErrorMessage(error, "No se pudieron obtener tus notificaciones.")}
                </p>
            ) : null}

            {data ? (
                <>
                    {!data.available ? (
                        <p className="mt-4 text-sm text-muted-foreground">
                            Un administrador aún no ha activado las notificaciones de Telegram.
                        </p>
                    ) : null}

                    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3">
                        <p className="text-sm text-foreground">
                            {link?.status === "linked" && !link.needsRelink ? `Conectado ${accountName ?? ""}` : null}
                            {link?.status === "blocked" && !link.needsRelink
                                ? "Bloqueaste al bot. Desbloquéalo en Telegram y envía un mensaje de prueba."
                                : null}
                            {hasLink && link?.needsRelink ? "El bot cambió: vincula tu cuenta otra vez." : null}
                            {!hasLink ? "Sin vincular" : null}
                        </p>
                        <div className="flex flex-wrap gap-2">
                            {data.available && !linked && link?.status !== "blocked" ? (
                                <Button size="sm" onClick={requestLink} disabled={startLink.isPending}>
                                    Vincular Telegram
                                </Button>
                            ) : null}
                            {data.available && hasLink && !link?.needsRelink ? (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={sendTest.isPending}
                                    onClick={() =>
                                        run(
                                            () => sendTest.mutateAsync(),
                                            "Mensaje de prueba enviado.",
                                            "No se pudo enviar la prueba.",
                                        )
                                    }
                                >
                                    <SendIcon />
                                    Enviar prueba
                                </Button>
                            ) : null}
                            {hasLink ? (
                                <Button
                                    size="sm"
                                    variant="ghost"
                                    disabled={unlink.isPending}
                                    onClick={() =>
                                        run(() => unlink.mutateAsync(), "Cuenta desvinculada.", "No se pudo desvincular.")
                                    }
                                >
                                    Desvincular
                                </Button>
                            ) : null}
                        </div>
                    </div>

                    {pendingRequest ? (
                        <div className="mt-3 rounded-lg border border-dashed p-3 text-sm">
                            <p className="text-foreground">
                                Abre el enlace y pulsa <strong>Iniciar</strong> en Telegram. Vence en 10 minutos y solo
                                sirve una vez; no lo compartas.
                            </p>
                            <a
                                className="mt-2 inline-flex items-center gap-1 font-medium text-primary underline"
                                href={pendingRequest.url}
                                target="_blank"
                                rel="noreferrer"
                            >
                                Abrir Telegram
                                <ExternalLinkIcon className="size-4" />
                            </a>
                            <p className="mt-2 text-xs text-muted-foreground" role="status">
                                Esperando la vinculación...
                            </p>
                        </div>
                    ) : null}

                    {notice ? (
                        <p
                            className={`mt-3 text-sm ${notice.tone === "error" ? "text-destructive" : "text-muted-foreground"}`}
                            role="status"
                        >
                            {notice.text}
                        </p>
                    ) : null}

                    {hasLink ? (
                        <label className="mt-4 flex items-center gap-2 text-sm font-medium text-foreground">
                            <input
                                type="checkbox"
                                className="size-4 accent-primary"
                                checked={data.notificationsEnabled}
                                disabled={setEnabled.isPending}
                                onChange={(event) =>
                                    run(
                                        () => setEnabled.mutateAsync(event.target.checked),
                                        null,
                                        "No se pudo guardar el cambio.",
                                    )
                                }
                            />
                            Recibir notificaciones
                        </label>
                    ) : null}

                    {data.preferences.length === 0 ? (
                        <p className="mt-4 text-sm text-muted-foreground">Tu rol no tiene avisos disponibles.</p>
                    ) : (
                        <>
                            <p className="mt-4 text-xs text-muted-foreground">
                                El resumen diario se envía a las {data.dailySummaryTime} ({data.timezone}).
                            </p>
                            {groupPreferences(data.preferences).map(([group, preferences]) => (
                                <fieldset key={group} className="mt-4">
                                    <legend className="text-sm font-semibold text-foreground">{group}</legend>
                                    <ul className="mt-2 space-y-3">
                                        {preferences.map((preference) => (
                                            <li key={preference.type}>
                                                <label className="flex items-start gap-2 text-sm">
                                                    <input
                                                        type="checkbox"
                                                        className="mt-0.5 size-4 accent-primary"
                                                        checked={preference.enabled}
                                                        onChange={(event) =>
                                                            run(
                                                                () =>
                                                                    savePreference.mutateAsync({
                                                                        type: preference.type,
                                                                        enabled: event.target.checked,
                                                                    }),
                                                                null,
                                                                "No se pudo guardar el aviso.",
                                                            )
                                                        }
                                                    />
                                                    <span className="flex-1">
                                                        <span className="block text-foreground">{preference.label}</span>
                                                        <span className="block text-xs text-muted-foreground">
                                                            {preference.description}
                                                        </span>
                                                    </span>
                                                    <span className="shrink-0 text-xs text-muted-foreground">
                                                        {preference.active
                                                            ? (DELIVERY_LABELS[preference.delivery] ?? preference.delivery)
                                                            : "Próximamente"}
                                                    </span>
                                                </label>
                                            </li>
                                        ))}
                                    </ul>
                                </fieldset>
                            ))}
                        </>
                    )}
                </>
            ) : null}
        </section>
    )
}

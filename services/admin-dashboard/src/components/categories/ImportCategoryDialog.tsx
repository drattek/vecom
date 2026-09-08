import { ExternalCategoryTree } from "@/components/categories/ExternalCategoryTree"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { getServerErrorMessage } from "@/lib/api"
import { useCategoryImportSources, useImportCategory } from "@/lib/categories"
import type { CategoryImportNode, CategoryImportSource } from "@/lib/schemas/categories"
import { DownloadIcon } from "lucide-react"
import { useMemo, useState } from "react"

export function ImportCategoryDialog() {
    const [open, setOpen] = useState(false)

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger
                render={
                    <Button variant="outline" size="sm">
                        <DownloadIcon />
                        Importar categoría
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-[46vw] min-w-88 max-w-none">
                {open ? <ImportBody onClose={() => setOpen(false)} /> : null}
            </DialogContent>
        </Dialog>
    )
}

function ImportBody({ onClose }: { onClose: () => void }) {
    const sourcesQuery = useCategoryImportSources()
    const sources = useMemo(() => sourcesQuery.data ?? [], [sourcesQuery.data])

    const [channelOverride, setChannelOverride] = useState<number | null>(null)
    const [connectionOverride, setConnectionOverride] = useState<number | null>(null)

    // Selección efectiva derivada: primer canal soportado / conexión única,
    // salvo que el usuario haya elegido explícitamente otra.
    const firstSupported = sources.find((source) => source.supported) ?? null
    const channelId = channelOverride ?? firstSupported?.channelId ?? null
    const selectedChannel = sources.find((source) => source.channelId === channelId) ?? null
    const connectionId =
        connectionOverride ??
        (selectedChannel && selectedChannel.connections.length === 1
            ? selectedChannel.connections[0].id
            : null)

    return (
        <>
            <DialogHeader>
                <DialogTitle>Importar categoría desde un canal</DialogTitle>
                <DialogDescription>
                    Navega el árbol de categorías del canal e importa una hoja. Se crean localmente la hoja y
                    los padres que falten, y se registra el mapeo.
                </DialogDescription>
            </DialogHeader>

            {sourcesQuery.isLoading ? (
                <p className="py-6 text-center text-sm text-muted-foreground">Cargando canales…</p>
            ) : sourcesQuery.isError ? (
                <p className="py-6 text-center text-sm text-destructive">No se pudieron cargar los canales.</p>
            ) : sources.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">
                    No hay canales con conexiones activas.
                </p>
            ) : (
                <div className="flex min-h-0 flex-1 flex-col gap-4">
                    <div className="grid gap-3 sm:grid-cols-2">
                        <div className="flex flex-col gap-1.5">
                            <Label>Canal</Label>
                            <ChannelSelect
                                sources={sources}
                                value={channelId}
                                onChange={(id) => {
                                    setChannelOverride(id)
                                    setConnectionOverride(null)
                                }}
                            />
                        </div>
                        <div className="flex flex-col gap-1.5">
                            <Label>Conexión</Label>
                            <ConnectionSelect
                                channel={selectedChannel}
                                value={connectionId}
                                onChange={setConnectionOverride}
                            />
                        </div>
                    </div>

                    {selectedChannel && !selectedChannel.supported ? (
                        <p className="text-sm text-muted-foreground">
                            La importación de categorías no está disponible para este canal.
                        </p>
                    ) : connectionId != null ? (
                        <ImportTree key={connectionId} connectionId={connectionId} />
                    ) : (
                        <p className="text-sm text-muted-foreground">Elige una conexión para navegar el árbol.</p>
                    )}
                </div>
            )}

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose}>
                    Cerrar
                </Button>
            </DialogFooter>
        </>
    )
}

function ImportTree({ connectionId }: { connectionId: number }) {
    const [onlySelected, setOnlySelected] = useState(false)
    const importMutation = useImportCategory()
    const [pickingId, setPickingId] = useState<string | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [lastResult, setLastResult] = useState<{ name: string; created: number; mapped: number } | null>(null)

    async function importNode(node: CategoryImportNode) {
        setPickingId(node.externalId)
        setError(null)
        try {
            const result = await importMutation.mutateAsync({
                connectionId,
                externalCategoryId: node.externalId,
                onlySelectedConnection: onlySelected,
            })
            setLastResult({
                name: result.name,
                created: result.createdCount,
                mapped: result.mappedConnectionIds.length,
            })
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo importar la categoría."))
        } finally {
            setPickingId(null)
        }
    }

    return (
        <div className="flex min-h-0 flex-1 flex-col gap-3">
            <label className="flex items-center gap-2 text-sm text-foreground">
                <input
                    type="checkbox"
                    checked={onlySelected}
                    onChange={(event) => setOnlySelected(event.target.checked)}
                    className="size-4 rounded border-input accent-primary"
                />
                Aplicar el mapeo solo a la conexión seleccionada
            </label>

            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            {lastResult ? (
                <p className="rounded-md border border-emerald-600/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-700 dark:text-emerald-400">
                    Se importó «{lastResult.name}» · {lastResult.created} categoría(s) nueva(s) · mapeada en{" "}
                    {lastResult.mapped} conexión(es).
                </p>
            ) : null}

            <ExternalCategoryTree
                connectionId={connectionId}
                onPick={importNode}
                pickLabel="Importar"
                pickingExternalId={pickingId}
                busy={importMutation.isPending}
            />
        </div>
    )
}

function ChannelSelect({
    sources,
    value,
    onChange,
}: {
    sources: CategoryImportSource[]
    value: number | null
    onChange: (id: number) => void
}) {
    const items = sources.map((source) => ({
        value: String(source.channelId),
        label: source.supported ? source.channelName : `${source.channelName} · no disponible`,
    }))
    return (
        <Select
            items={items}
            value={value != null ? String(value) : ""}
            onValueChange={(next) => next && onChange(Number(next))}
        >
            <SelectTrigger className="w-full">
                <SelectValue placeholder="Elige un canal" />
            </SelectTrigger>
            <SelectContent align="start">
                <SelectGroup>
                    {sources.map((source) => (
                        <SelectItem
                            key={source.channelId}
                            value={String(source.channelId)}
                            disabled={!source.supported}
                        >
                            {source.channelName}
                            {source.supported ? "" : " · no disponible"}
                        </SelectItem>
                    ))}
                </SelectGroup>
            </SelectContent>
        </Select>
    )
}

function ConnectionSelect({
    channel,
    value,
    onChange,
}: {
    channel: CategoryImportSource | null
    value: number | null
    onChange: (id: number) => void
}) {
    const connections = channel?.connections ?? []
    const items = connections.map((connection) => ({
        value: String(connection.id),
        label:
            connection.environment === "production"
                ? connection.name
                : `${connection.name} · desarrollo`,
    }))
    return (
        <Select
            items={items}
            value={value != null ? String(value) : ""}
            onValueChange={(next) => next && onChange(Number(next))}
            disabled={!channel || !channel.supported || connections.length === 0}
        >
            <SelectTrigger className="w-full">
                <SelectValue placeholder="Elige una conexión" />
            </SelectTrigger>
            <SelectContent align="start">
                <SelectGroup>
                    {connections.map((connection) => (
                        <SelectItem key={connection.id} value={String(connection.id)}>
                            {connection.name}
                            {connection.environment === "production" ? "" : " · desarrollo"}
                        </SelectItem>
                    ))}
                </SelectGroup>
            </SelectContent>
        </Select>
    )
}

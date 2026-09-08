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
import { getServerErrorMessage } from "@/lib/api"
import { useSetCategoryChannelMapping } from "@/lib/categories"
import type { CategoryChannelMapping, CategoryImportNode } from "@/lib/schemas/categories"
import { Link2Icon } from "lucide-react"
import { useState } from "react"

/**
 * Vincula (o reemplaza) el mapeo de la categoría local que se está viendo con
 * una hoja de categoría externa de una conexión, navegando su árbol. No crea
 * categorías locales: solo escribe ecom_channel_category_map.
 */
export function LinkCategoryMappingDialog({
    categoryId,
    mapping,
}: {
    categoryId: string | undefined
    mapping: CategoryChannelMapping
}) {
    const [open, setOpen] = useState(false)
    const isReplace = Boolean(mapping.externalCategoryId)

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger
                render={
                    <Button variant={isReplace ? "ghost" : "outline"} size="sm" className="h-7 px-2 text-xs">
                        {isReplace ? null : <Link2Icon className="size-3.5" />}
                        {isReplace ? "Reemplazar" : "Vincular categoría externa"}
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-[46vw] min-w-88 max-w-none">
                {open ? (
                    <LinkBody
                        categoryId={categoryId}
                        mapping={mapping}
                        isReplace={isReplace}
                        onClose={() => setOpen(false)}
                    />
                ) : null}
            </DialogContent>
        </Dialog>
    )
}

function LinkBody({
    categoryId,
    mapping,
    isReplace,
    onClose,
}: {
    categoryId: string | undefined
    mapping: CategoryChannelMapping
    isReplace: boolean
    onClose: () => void
}) {
    const mutation = useSetCategoryChannelMapping(categoryId)
    const [pickingId, setPickingId] = useState<string | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [done, setDone] = useState<{ name: string; replaced: boolean } | null>(null)

    async function pick(node: CategoryImportNode) {
        setPickingId(node.externalId)
        setError(null)
        try {
            const result = await mutation.mutateAsync({
                connectionId: mapping.connectionId,
                externalCategoryId: node.externalId,
            })
            setDone({ name: result.externalCategoryName || node.name, replaced: result.replaced })
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar el mapeo."))
        } finally {
            setPickingId(null)
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{isReplace ? "Reemplazar mapeo" : "Vincular categoría externa"}</DialogTitle>
                <DialogDescription>
                    {mapping.channelName} · {mapping.connectionName}
                    {isReplace && mapping.externalCategoryName
                        ? ` — actual: ${mapping.externalCategoryName} (${mapping.externalCategoryId})`
                        : ""}
                </DialogDescription>
            </DialogHeader>

            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            {done ? (
                <p className="rounded-md border border-emerald-600/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-700 dark:text-emerald-400">
                    {done.replaced ? "Mapeo reemplazado" : "Categoría vinculada"}: «{done.name}».
                </p>
            ) : null}

            <ExternalCategoryTree
                connectionId={mapping.connectionId}
                onPick={pick}
                pickLabel={isReplace ? "Usar esta" : "Vincular"}
                pickingExternalId={pickingId}
                busy={mutation.isPending}
            />

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose}>
                    {done ? "Cerrar" : "Cancelar"}
                </Button>
            </DialogFooter>
        </>
    )
}

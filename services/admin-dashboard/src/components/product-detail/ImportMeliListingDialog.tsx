import { Badge } from "@/components/product-detail/ProductDetailPrimitives"
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
import { getServerErrorMessage } from "@/lib/api"
import { useImportMeliListing } from "@/lib/product-detail"
import type { ImportListingResult, ImportListingStep } from "@/lib/schemas/product-details"
import axios from "axios"
import { useState } from "react"
import { ZodError } from "zod"

/**
 * Motivo legible de un fallo al importar. getServerErrorMessage devuelve su
 * texto de respaldo cuando no hay mensaje del servidor (timeout, red caída, una
 * respuesta que no cumple el schema), que es justo cuando más hace falta saber
 * qué pasó; aquí cada caso se nombra y el detalle técnico también va a la consola.
 */
function importErrorMessage(error: unknown): string {
    console.error("Error al importar la publicación", error)

    if (axios.isAxiosError(error)) {
        if (!error.response) {
            return error.code === "ECONNABORTED"
                ? "El servidor tardó demasiado en responder. La importación pudo haber avanzado parcialmente: revisa el producto y vuelve a intentar (es seguro repetirla)."
                : `No hubo respuesta del servidor (${error.message}). Revisa tu conexión o que el servicio esté en línea.`
        }
        const status = error.response.status
        const serverMessage = getServerErrorMessage(error, "")
        return serverMessage ? `${serverMessage} (error ${status})` : `El servidor respondió con el error ${status} sin detalle.`
    }

    if (error instanceof ZodError) {
        const detail = error.issues
            .slice(0, 3)
            .map((issue) => `${issue.path.join(".") || "respuesta"}: ${issue.message}`)
            .join("; ")
        return `El servidor respondió con un formato inesperado (${detail}). La importación pudo haberse completado: recarga la sección.`
    }

    return error instanceof Error ? `No se pudo importar la publicación: ${error.message}` : "No se pudo importar la publicación."
}

const stepLabels: Record<string, string> = {
    name: "Nombre del producto",
    category: "Categoría de la conexión",
    description: "Descripción",
    attributes: "Atributos",
    compatibilities: "Compatibilidades",
    images: "Imágenes",
    link: "Vínculo con la publicación",
    price_stock: "Precio y stock",
    title: "Título con marca",
}

const statusLabels: Record<ImportListingStep["status"], string> = {
    ok: "Listo",
    skipped: "Omitido",
    warning: "Aviso",
    error: "Error",
}

const statusTones: Record<ImportListingStep["status"], "success" | "muted" | "warning"> = {
    ok: "success",
    skipped: "muted",
    warning: "warning",
    error: "warning",
}

/**
 * Importa una publicación que ya existe en MercadoLibre y la vincula al
 * producto. El core valida que el número de parte de la publicación coincida
 * con el del producto antes de tocar nada; si no coincide (o la publicación no
 * es de la cuenta de la conexión, ya está vinculada a otro producto, etc.) el
 * error llega como mensaje y no se guarda nada.
 */
export function ImportMeliListingDialog({
    productId,
    connectionId,
    open,
    onOpenChange,
}: {
    productId?: string
    connectionId: number
    open: boolean
    onOpenChange: (open: boolean) => void
}) {
    const mutation = useImportMeliListing(productId)
    const [externalId, setExternalId] = useState("")
    const [result, setResult] = useState<ImportListingResult | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    const trimmedId = externalId.trim()
    const isDone = result?.linked === true

    function handleOpenChange(next: boolean) {
        if (mutation.isPending) {
            return
        }
        if (!next) {
            setExternalId("")
            setResult(null)
            setErrorMessage(null)
        }
        onOpenChange(next)
    }

    function submit() {
        if (!trimmedId || !productId) {
            return
        }
        setResult(null)
        setErrorMessage(null)
        mutation.mutate(
            { connectionId, externalId: trimmedId },
            {
                onSuccess: setResult,
                onError: (error) => setErrorMessage(importErrorMessage(error)),
            },
        )
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-xl">
                <DialogHeader>
                    <DialogTitle>Importar publicación existente</DialogTitle>
                    <DialogDescription>
                        Pega el ID de la publicación de MercadoLibre (por ejemplo MLM123456789) o su enlace. Se valida que el
                        número de parte coincida con el del producto antes de importar.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="import-meli-listing-id">ID de la publicación</Label>
                        <Input
                            id="import-meli-listing-id"
                            value={externalId}
                            onChange={(event) => setExternalId(event.target.value)}
                            onKeyDown={(event) => {
                                if (event.key === "Enter" && !isDone) {
                                    submit()
                                }
                            }}
                            placeholder="MLM123456789"
                            autoComplete="off"
                            autoFocus
                            disabled={mutation.isPending || isDone}
                        />
                    </div>

                    {!result && !errorMessage ? (
                        <p className="text-xs text-muted-foreground">
                            Se actualizarán el nombre, la categoría de esta conexión, la descripción, los atributos y las
                            compatibilidades del producto con lo que tiene la publicación, y se agregarán sus imágenes
                            (convertidas a WebP) a las que ya tenga. Si el producto tiene stock y precio, también se enviarán
                            el precio y el stock, y se agregará la marca al título. Puede tardar un par de minutos.
                        </p>
                    ) : null}

                    {errorMessage ? (
                        <p className="text-sm text-destructive" role="alert">
                            {errorMessage}
                        </p>
                    ) : null}

                    {result ? <ImportResult result={result} /> : null}
                </div>

                <DialogFooter>
                    <Button variant="ghost" size="sm" onClick={() => handleOpenChange(false)} disabled={mutation.isPending}>
                        {isDone ? "Cerrar" : "Cancelar"}
                    </Button>
                    {!isDone ? (
                        <Button size="sm" onClick={submit} disabled={mutation.isPending || !trimmedId || !productId}>
                            {mutation.isPending ? "Importando…" : result ? "Reintentar" : "Importar"}
                        </Button>
                    ) : null}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ImportResult({ result }: { result: ImportListingResult }) {
    return (
        <div className="flex flex-col gap-2">
            <p className={result.linked ? "text-sm font-medium text-foreground" : "text-sm font-medium text-destructive"}>
                {result.linked
                    ? `Publicación ${result.externalId} vinculada al producto.`
                    : "La publicación no se vinculó: revisa los errores y vuelve a intentar."}
            </p>
            <ul className="flex flex-col gap-1.5">
                {result.steps.map((step) => (
                    <li key={step.step} className="flex flex-col gap-0.5 rounded-md border border-border px-3 py-2 text-sm">
                        <span className="flex items-center justify-between gap-2">
                            <span className="font-medium text-foreground">{stepLabels[step.step] ?? step.step}</span>
                            <Badge tone={statusTones[step.status]}>{statusLabels[step.status]}</Badge>
                        </span>
                        {step.detail ? (
                            <span className={step.status === "error" ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
                                {step.detail}
                            </span>
                        ) : null}
                    </li>
                ))}
            </ul>
        </div>
    )
}

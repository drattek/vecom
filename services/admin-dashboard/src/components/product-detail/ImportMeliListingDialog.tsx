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
import { useImportMeliListing, useImportMeliListingJob } from "@/lib/product-detail"
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
        // 502/503/504 sin mensaje del API: lo generó un proxy (nginx, el ingress)
        // al cansarse de esperar, no el servicio. El backend sigue importando.
        if (!serverMessage && (status === 502 || status === 503 || status === 504)) {
            return `Un servidor intermedio cortó la espera (error ${status}), pero la importación sigue ejecutándose en el servidor. Espera uno o dos minutos y recarga la sección para ver el resultado; es seguro repetirla.`
        }
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
    listing_type: "Tipo de publicación Premium",
    official_store: "Fuera de tienda oficial",
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
    const [jobId, setJobId] = useState<string | null>(null)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)
    // La importación corre en segundo plano en el servidor; aquí solo se consulta
    // su avance (ver useImportMeliListingJob).
    const jobQuery = useImportMeliListingJob(productId, jobId)

    const trimmedId = externalId.trim()
    const job = jobQuery.data
    const result: ImportListingResult | null = job?.result ?? null
    // Corriendo desde que se pulsa "Importar" hasta que el job termina.
    const isRunning = mutation.isPending || (jobId !== null && !jobQuery.isError && (!job || job.status === "running"))
    const isFinished = job?.status === "done" || job?.status === "failed"
    const isDone = result?.linked === true && isFinished
    const failureMessage =
        errorMessage ??
        (job?.status === "failed" && job.error ? job.error : null) ??
        (jobQuery.isError ? importErrorMessage(jobQuery.error) : null)

    function handleOpenChange(next: boolean) {
        // Solo se bloquea mientras se arranca la importación; una vez en marcha
        // sigue en el servidor aunque se cierre el diálogo.
        if (mutation.isPending) {
            return
        }
        if (!next) {
            setExternalId("")
            setJobId(null)
            setErrorMessage(null)
        }
        onOpenChange(next)
    }

    function submit() {
        if (!trimmedId || !productId || isRunning) {
            return
        }
        setJobId(null)
        setErrorMessage(null)
        mutation.mutate(
            { connectionId, externalId: trimmedId },
            {
                onSuccess: (started) => setJobId(started.jobId),
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
                            disabled={isRunning || isDone}
                        />
                    </div>

                    {!jobId && !errorMessage ? (
                        <p className="text-xs text-muted-foreground">
                            Se actualizarán el nombre, la categoría de esta conexión, la descripción, los atributos y las
                            compatibilidades del producto con lo que tiene la publicación, y se agregarán sus imágenes
                            (convertidas a WebP) a las que ya tenga. Si el producto tiene stock y precio, también se enviarán
                            el precio y el stock, y se agregará la marca al título. Puede tardar un par de minutos.
                        </p>
                    ) : null}

                    {failureMessage ? (
                        <p className="text-sm text-destructive" role="alert">
                            {failureMessage}
                        </p>
                    ) : null}

                    {isRunning ? (
                        <p className="text-sm text-muted-foreground" role="status">
                            Importando… puede tardar un par de minutos (sobre todo por las imágenes). Puedes cerrar esta ventana:
                            la importación sigue en el servidor.
                        </p>
                    ) : null}

                    {result ? <ImportResult result={result} finished={isFinished} /> : null}
                </div>

                <DialogFooter>
                    <Button variant="ghost" size="sm" onClick={() => handleOpenChange(false)} disabled={mutation.isPending}>
                        {isDone || isFinished ? "Cerrar" : "Cancelar"}
                    </Button>
                    {!isDone ? (
                        <Button size="sm" onClick={submit} disabled={isRunning || !trimmedId || !productId}>
                            {isRunning ? "Importando…" : jobId || errorMessage ? "Reintentar" : "Importar"}
                        </Button>
                    ) : null}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ImportResult({ result, finished }: { result: ImportListingResult; finished: boolean }) {
    return (
        <div className="flex flex-col gap-2">
            {result.linked || finished ? (
                <p className={result.linked ? "text-sm font-medium text-foreground" : "text-sm font-medium text-destructive"}>
                    {result.linked
                        ? `Publicación ${result.externalId} vinculada al producto.`
                        : "La publicación no se vinculó: revisa los errores y vuelve a intentar."}
                </p>
            ) : null}
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

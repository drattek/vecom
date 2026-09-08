import {
    Badge,
    SectionCard,
    SectionState,
} from "@/components/product-detail/ProductDetailPrimitives"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { getServerErrorMessage } from "@/lib/api"
import {
    formatDate,
    formatFileSize,
    useAddProductAttachments,
    useAddProductImages,
    useAddProductVideos,
    useDeleteProductAttachment,
    useDeleteProductImage,
    useDeleteProductVideo,
    useReplaceProductAttachment,
    useReplaceProductImage,
    useReplaceProductVideo,
    useSetProductImageCover,
    useStorageDisks,
} from "@/lib/product-detail"
import type {
    ProductAttachmentImportResult,
    ProductImageImportResult,
    ProductMediaItem,
    ProductVideoImportResult,
} from "@/lib/schemas/product-details"
import { cn } from "@/lib/utils"
import {
    CheckIcon,
    ExternalLinkIcon,
    FilePlusIcon,
    FileTextIcon,
    ImagePlusIcon,
    PencilIcon,
    PlusIcon,
    RefreshCwIcon,
    StarIcon,
    Trash2Icon,
    VideoIcon,
    XIcon,
} from "lucide-react"
import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"

const DISK_STORAGE_KEY = "product-media-storage-disk"

const ATTACHMENT_TYPE_OPTIONS = [
    { value: "manual", label: "Manual" },
    { value: "datasheet", label: "Ficha técnica" },
    { value: "certificate", label: "Certificado" },
    { value: "image", label: "Imagen" },
] as const

const attachmentTypeLabels: Record<string, string> = Object.fromEntries(
    ATTACHMENT_TYPE_OPTIONS.map((option) => [option.value, option.label]),
)

function readStoredDiskId(): number | null {
    try {
        const raw = localStorage.getItem(DISK_STORAGE_KEY)
        const parsed = raw ? Number(raw) : NaN
        return Number.isFinite(parsed) && parsed > 0 ? parsed : null
    } catch {
        return null
    }
}

function storeDiskId(id: number) {
    try {
        localStorage.setItem(DISK_STORAGE_KEY, String(id))
    } catch {
        /* localStorage no disponible: se ignora */
    }
}

/** Divide el textarea en URLs limpias, sin vacíos ni repetidas, en orden. */
function parseUrls(raw: string): string[] {
    const seen = new Set<string>()
    const urls: string[] = []
    for (const line of raw.split(/\r?\n/)) {
        const url = line.trim()
        if (url && !seen.has(url)) {
            seen.add(url)
            urls.push(url)
        }
    }
    return urls
}

/** Selector de disco de almacenamiento, mostrando el nombre del disco. */
function StorageDiskSelect({
    value,
    onChange,
    disabled,
}: {
    value: number | null
    onChange: (id: number) => void
    disabled?: boolean
}) {
    const disks = useStorageDisks()
    const items = (disks.data ?? []).map((disk) => ({ label: disk.name, value: String(disk.id) }))

    return (
        <div className="flex flex-col gap-1">
            <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Disco de almacenamiento
            </label>
            <Select
                items={items}
                value={value != null ? String(value) : ""}
                onValueChange={(next) => {
                    const id = Number(next)
                    if (Number.isFinite(id) && id > 0) {
                        onChange(id)
                        storeDiskId(id)
                    }
                }}
                disabled={disabled || disks.isLoading || items.length === 0}
            >
                <SelectTrigger className="w-full" aria-label="Disco de almacenamiento">
                    <SelectValue placeholder={disks.isLoading ? "Cargando discos…" : "Elegí un disco"} />
                </SelectTrigger>
                <SelectContent align="start">
                    <SelectGroup>
                        {(disks.data ?? []).map((disk) => (
                            <SelectItem key={disk.id} value={String(disk.id)}>
                                {disk.name}
                                <span className="ml-1 text-xs text-muted-foreground">{disk.code}</span>
                            </SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
            {items.length === 0 && !disks.isLoading ? (
                <p className="text-xs text-destructive">
                    No hay discos de almacenamiento configurados.
                </p>
            ) : null}
        </div>
    )
}

/** Miniatura de una URL con estado de carga/error, para previsualizar antes de guardar. */
function UrlThumb({ url, className }: { url: string; className?: string }) {
    const [errored, setErrored] = useState(false)
    return (
        <div className={cn("relative aspect-square overflow-hidden rounded-md border border-border bg-muted", className)}>
            {errored ? (
                <div className="flex size-full items-center justify-center px-1 text-center text-[10px] text-muted-foreground">
                    No carga
                </div>
            ) : (
                <img
                    src={url}
                    alt={url}
                    loading="lazy"
                    onError={() => setErrored(true)}
                    className="size-full object-cover"
                />
            )}
        </div>
    )
}

function ResultLine({ result }: { result: ProductImageImportResult }) {
    return (
        <li className="flex items-start gap-2 text-xs">
            {result.success ? (
                <CheckIcon className="mt-0.5 size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
            ) : (
                <XIcon className="mt-0.5 size-3.5 shrink-0 text-destructive" />
            )}
            <span className="min-w-0 break-all">
                <span className="text-muted-foreground">{result.url}</span>
                {result.error ? <span className="text-destructive"> — {result.error}</span> : null}
            </span>
        </li>
    )
}

const KEEP_COVER = "__keep__"

function AddImagesDialog({
    productId,
    triggerVariant = "default",
}: {
    productId?: string
    triggerVariant?: "default" | "outline"
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [text, setText] = useState("")
    const [cover, setCover] = useState<string>(KEEP_COVER)
    const [results, setResults] = useState<ProductImageImportResult[] | null>(null)
    const [error, setError] = useState<string | null>(null)

    const mutation = useAddProductImages(productId)
    const urls = useMemo(() => parseUrls(text), [text])

    function reset() {
        setText("")
        setCover(KEEP_COVER)
        setResults(null)
        setError(null)
        mutation.reset()
    }

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            reset()
        }
    }

    async function submit() {
        setError(null)
        setResults(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        if (urls.length === 0) {
            setError("Agregá al menos una URL.")
            return
        }
        try {
            const response = await mutation.mutateAsync({
                storageDiskId: diskId,
                images: urls.map((url) => ({ url, isFirst: url === cover })),
            })
            setResults(response.results)
            if (response.results.every((r) => r.success)) {
                setOpen(false)
                reset()
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudieron agregar las imágenes."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant={triggerVariant} size="sm">
                        <ImagePlusIcon />
                        Agregar imágenes
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-xl max-w-[calc(100%-2rem)] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Agregar imágenes por URL</DialogTitle>
                    <DialogDescription>
                        Pegá una URL por línea. Cada URL se valida (debe responder y apuntar a una imagen)
                        antes de asociarla al producto.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />

                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            URLs
                        </label>
                        <Textarea
                            value={text}
                            onChange={(event) => {
                                setText(event.target.value)
                                setResults(null)
                            }}
                            rows={5}
                            placeholder={"https://cdn.ejemplo.com/img-1.jpg\nhttps://cdn.ejemplo.com/img-2.jpg"}
                            disabled={mutation.isPending}
                            aria-label="URLs de las imágenes"
                        />
                        {urls.length > 0 ? (
                            <p className="text-xs text-muted-foreground">
                                {urls.length} URL{urls.length === 1 ? "" : "s"} detectada
                                {urls.length === 1 ? "" : "s"}
                            </p>
                        ) : null}
                    </div>

                    {urls.length > 0 ? (
                        <div className="flex flex-col gap-2">
                            <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                Portada
                            </span>
                            <label className="flex items-center gap-2 text-sm">
                                <input
                                    type="radio"
                                    name="cover"
                                    checked={cover === KEEP_COVER}
                                    onChange={() => setCover(KEEP_COVER)}
                                    disabled={mutation.isPending}
                                />
                                Mantener la portada actual
                            </label>
                            <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
                                {urls.map((url) => (
                                    <label key={url} className="flex cursor-pointer flex-col gap-1.5">
                                        <UrlThumb url={url} />
                                        <span className="flex items-center gap-1.5 text-xs">
                                            <input
                                                type="radio"
                                                name="cover"
                                                checked={cover === url}
                                                onChange={() => setCover(url)}
                                                disabled={mutation.isPending}
                                            />
                                            Portada
                                        </span>
                                    </label>
                                ))}
                            </div>
                        </div>
                    ) : null}

                    {results ? (
                        <ul className="flex flex-col gap-1.5 rounded-md border border-border bg-muted/40 p-3">
                            {results.map((result) => (
                                <ResultLine key={result.url} result={result} />
                            ))}
                        </ul>
                    ) : null}

                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cerrar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending || urls.length === 0}>
                        {mutation.isPending ? "Agregando…" : "Agregar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ReplaceImageDialog({
    productId,
    image,
}: {
    productId?: string
    image: ProductMediaItem
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [url, setUrl] = useState("")
    const [error, setError] = useState<string | null>(null)

    const mutation = useReplaceProductImage(productId)

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            setUrl("")
            setError(null)
            mutation.reset()
        }
    }

    async function submit() {
        setError(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        const trimmed = url.trim()
        if (!trimmed) {
            setError("Ingresá la URL de la nueva imagen.")
            return
        }
        try {
            const result = await mutation.mutateAsync({ imageId: image.id, storageDiskId: diskId, url: trimmed })
            if (result.success) {
                handleOpenChange(false)
            } else {
                setError(result.error ?? "No se pudo reemplazar la imagen.")
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo reemplazar la imagen."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant="ghost" size="xs" aria-label="Reemplazar imagen">
                        <RefreshCwIcon />
                        Reemplazar
                    </Button>
                }
            />
            <DialogContent className="w-120 max-w-[calc(100%-2rem)]">
                <DialogHeader>
                    <DialogTitle>Reemplazar imagen</DialogTitle>
                    <DialogDescription>
                        La imagen actual se elimina y se asocia la nueva URL, conservando si era la portada.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />
                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Nueva URL
                        </label>
                        <Input
                            value={url}
                            onChange={(event) => setUrl(event.target.value)}
                            placeholder="https://cdn.ejemplo.com/nueva.jpg"
                            disabled={mutation.isPending}
                            aria-label="Nueva URL de la imagen"
                        />
                    </div>
                    {url.trim() ? <UrlThumb url={url.trim()} className="w-32" /> : null}
                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cancelar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Reemplazando…" : "Reemplazar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ProductImageTile({
    productId,
    image,
    managing,
}: {
    productId?: string
    image: ProductMediaItem
    managing: boolean
}) {
    const [errored, setErrored] = useState(false)
    const [confirmingDelete, setConfirmingDelete] = useState(false)

    const coverMutation = useSetProductImageCover(productId)
    const deleteMutation = useDeleteProductImage(productId)
    const busy = coverMutation.isPending || deleteMutation.isPending

    return (
        <figure className="overflow-hidden rounded-md border border-border">
            <div className="relative aspect-square bg-muted">
                {image.url && !errored ? (
                    <img
                        src={image.url}
                        alt={image.filename}
                        loading="lazy"
                        onError={() => setErrored(true)}
                        className="size-full object-cover"
                    />
                ) : (
                    <div className="flex size-full items-center justify-center text-xs text-muted-foreground">
                        No disponible
                    </div>
                )}
                {image.isFirst ? (
                    <span className="absolute left-1.5 top-1.5">
                        <Badge tone="success">Portada</Badge>
                    </span>
                ) : null}
            </div>

            <figcaption className="border-t border-border px-2 py-1.5">
                <p className="truncate text-xs font-medium text-foreground" title={image.filename}>
                    {image.filename}
                </p>
                <p className="text-xs text-muted-foreground">
                    {image.extension.toUpperCase()} · {formatFileSize(image.size)}
                    {image.width && image.height ? ` · ${image.width}×${image.height}` : ""}
                </p>

                {managing ? (
                    <div className="mt-1.5 flex flex-wrap items-center gap-1 border-t border-border pt-1.5">
                        {confirmingDelete ? (
                            <>
                                <span className="text-xs text-muted-foreground">¿Eliminar?</span>
                                <Button
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setConfirmingDelete(false)}
                                    disabled={busy}
                                >
                                    No
                                </Button>
                                <Button
                                    variant="destructive"
                                    size="xs"
                                    onClick={() => deleteMutation.mutate(image.id)}
                                    disabled={busy}
                                >
                                    {deleteMutation.isPending ? "Eliminando…" : "Sí, eliminar"}
                                </Button>
                            </>
                        ) : (
                            <>
                                {!image.isFirst ? (
                                    <Button
                                        variant="ghost"
                                        size="xs"
                                        onClick={() => coverMutation.mutate(image.id)}
                                        disabled={busy}
                                    >
                                        <StarIcon />
                                        {coverMutation.isPending ? "Guardando…" : "Portada"}
                                    </Button>
                                ) : null}
                                <ReplaceImageDialog productId={productId} image={image} />
                                <Button
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setConfirmingDelete(true)}
                                    disabled={busy}
                                >
                                    <Trash2Icon />
                                    Eliminar
                                </Button>
                            </>
                        )}
                    </div>
                ) : null}
            </figcaption>
        </figure>
    )
}

/**
 * Card "Imágenes" del detalle de producto: grilla de solo lectura + un modo
 * "Editar" que habilita agregar imágenes por URL, elegir la portada, reemplazar
 * y eliminar. Reemplaza el bloque estático que tenía ProductMediaSection.
 */
export function ProductImagesCard({ images }: { images: ProductMediaItem[] }) {
    const { productId } = useParams<{ productId: string }>()
    const [managing, setManaging] = useState(false)

    return (
        <SectionCard
            title="Imágenes"
            description="La imagen marcada como portada es la que se publica primero en los canales."
            action={
                <div className="flex items-center gap-1.5">
                    {managing && images.length > 0 ? <AddImagesDialog productId={productId} /> : null}
                    <Button variant="ghost" size="sm" onClick={() => setManaging((value) => !value)}>
                        {managing ? (
                            "Listo"
                        ) : (
                            <>
                                <PencilIcon />
                                Editar
                            </>
                        )}
                    </Button>
                </div>
            }
        >
            {images.length === 0 ? (
                managing ? (
                    <div className="flex flex-col items-center gap-3 py-6">
                        <p className="text-sm text-muted-foreground">Este producto no tiene imágenes cargadas.</p>
                        <AddImagesDialog productId={productId} triggerVariant="outline" />
                    </div>
                ) : (
                    <SectionState state="empty" message="Este producto no tiene imágenes cargadas." />
                )
            ) : (
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5 xl:grid-cols-6">
                    {images.map((image) => (
                        <ProductImageTile
                            key={image.id}
                            productId={productId}
                            image={image}
                            managing={managing}
                        />
                    ))}
                </div>
            )}
        </SectionCard>
    )
}

// --- Archivos adjuntos ---------------------------------------------------------

function AttachmentTypeSelect({
    value,
    onChange,
    disabled,
}: {
    value: string
    onChange: (type: string) => void
    disabled?: boolean
}) {
    return (
        <Select
            items={ATTACHMENT_TYPE_OPTIONS.map((option) => ({ label: option.label, value: option.value }))}
            value={value}
            onValueChange={(next) => onChange(next ?? "")}
            disabled={disabled}
        >
            <SelectTrigger className="w-40" aria-label="Tipo de archivo">
                <SelectValue placeholder="Tipo" />
            </SelectTrigger>
            <SelectContent align="start">
                <SelectGroup>
                    {ATTACHMENT_TYPE_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                            {option.label}
                        </SelectItem>
                    ))}
                </SelectGroup>
            </SelectContent>
        </Select>
    )
}

type AttachmentDraft = { url: string; type: string }

function AddAttachmentsDialog({
    productId,
    triggerVariant = "default",
}: {
    productId?: string
    triggerVariant?: "default" | "outline"
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [rows, setRows] = useState<AttachmentDraft[]>([{ url: "", type: "manual" }])
    const [results, setResults] = useState<ProductAttachmentImportResult[] | null>(null)
    const [error, setError] = useState<string | null>(null)

    const mutation = useAddProductAttachments(productId)

    function reset() {
        setRows([{ url: "", type: "manual" }])
        setResults(null)
        setError(null)
        mutation.reset()
    }

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            reset()
        }
    }

    function updateRow(index: number, patch: Partial<AttachmentDraft>) {
        setRows((prev) => prev.map((row, i) => (i === index ? { ...row, ...patch } : row)))
        setResults(null)
    }

    const validRows = rows.map((row) => ({ ...row, url: row.url.trim() })).filter((row) => row.url)

    async function submit() {
        setError(null)
        setResults(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        if (validRows.length === 0) {
            setError("Agregá al menos un archivo con su URL.")
            return
        }
        try {
            const response = await mutation.mutateAsync({ storageDiskId: diskId, attachments: validRows })
            setResults(response.results)
            if (response.results.every((r) => r.success)) {
                handleOpenChange(false)
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudieron agregar los archivos."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant={triggerVariant} size="sm">
                        <FilePlusIcon />
                        Agregar archivos
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-xl max-w-[calc(100%-2rem)] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Agregar archivos por URL</DialogTitle>
                    <DialogDescription>
                        Cada archivo se valida (debe responder) y se registra en el producto con el tipo
                        que elijas. Se toma la URL donde ya está alojado, no se sube el archivo.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />

                    <div className="flex flex-col gap-2">
                        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Archivos
                        </span>
                        {rows.map((row, index) => (
                            <div key={index} className="flex items-start gap-2">
                                <Input
                                    value={row.url}
                                    onChange={(event) => updateRow(index, { url: event.target.value })}
                                    placeholder="https://cdn.ejemplo.com/manual.pdf"
                                    disabled={mutation.isPending}
                                    aria-label={`URL del archivo ${index + 1}`}
                                    className="flex-1"
                                />
                                <AttachmentTypeSelect
                                    value={row.type}
                                    onChange={(type) => updateRow(index, { type })}
                                    disabled={mutation.isPending}
                                />
                                <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    onClick={() => setRows((prev) => prev.filter((_, i) => i !== index))}
                                    disabled={mutation.isPending || rows.length === 1}
                                    aria-label="Quitar fila"
                                >
                                    <XIcon />
                                </Button>
                            </div>
                        ))}
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setRows((prev) => [...prev, { url: "", type: "manual" }])}
                            disabled={mutation.isPending}
                            className="self-start"
                        >
                            <PlusIcon />
                            Agregar otra fila
                        </Button>
                    </div>

                    {results ? (
                        <ul className="flex flex-col gap-1.5 rounded-md border border-border bg-muted/40 p-3">
                            {results.map((result, index) => (
                                <li key={`${result.url}-${index}`} className="flex items-start gap-2 text-xs">
                                    {result.success ? (
                                        <CheckIcon className="mt-0.5 size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                                    ) : (
                                        <XIcon className="mt-0.5 size-3.5 shrink-0 text-destructive" />
                                    )}
                                    <span className="min-w-0 break-all">
                                        <span className="text-muted-foreground">{result.url}</span>
                                        {result.error ? (
                                            <span className="text-destructive"> — {result.error}</span>
                                        ) : null}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    ) : null}

                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cerrar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending || validRows.length === 0}>
                        {mutation.isPending ? "Agregando…" : "Agregar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ReplaceAttachmentDialog({
    productId,
    attachment,
}: {
    productId?: string
    attachment: ProductMediaItem
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [url, setUrl] = useState("")
    const [type, setType] = useState(attachment.type || "manual")
    const [error, setError] = useState<string | null>(null)

    const mutation = useReplaceProductAttachment(productId)

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            setUrl("")
            setType(attachment.type || "manual")
            setError(null)
            mutation.reset()
        }
    }

    async function submit() {
        setError(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        const trimmed = url.trim()
        if (!trimmed) {
            setError("Ingresá la URL del nuevo archivo.")
            return
        }
        try {
            const result = await mutation.mutateAsync({
                fileId: attachment.id,
                storageDiskId: diskId,
                url: trimmed,
                type,
            })
            if (result.success) {
                handleOpenChange(false)
            } else {
                setError(result.error ?? "No se pudo reemplazar el archivo.")
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo reemplazar el archivo."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant="ghost" size="xs" aria-label="Reemplazar archivo">
                        <RefreshCwIcon />
                        Reemplazar
                    </Button>
                }
            />
            <DialogContent className="w-120 max-w-[calc(100%-2rem)]">
                <DialogHeader>
                    <DialogTitle>Reemplazar archivo</DialogTitle>
                    <DialogDescription>
                        El archivo actual se elimina y se asocia la nueva URL con el tipo elegido.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />
                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Nueva URL
                        </label>
                        <Input
                            value={url}
                            onChange={(event) => setUrl(event.target.value)}
                            placeholder="https://cdn.ejemplo.com/nuevo.pdf"
                            disabled={mutation.isPending}
                            aria-label="Nueva URL del archivo"
                        />
                    </div>
                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Tipo
                        </label>
                        <AttachmentTypeSelect value={type} onChange={setType} disabled={mutation.isPending} />
                    </div>
                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cancelar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Reemplazando…" : "Reemplazar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function AttachmentRow({
    productId,
    attachment,
    managing,
}: {
    productId?: string
    attachment: ProductMediaItem
    managing: boolean
}) {
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    const deleteMutation = useDeleteProductAttachment(productId)

    return (
        <TableRow>
            <TableCell className="flex items-center gap-2">
                <FileTextIcon className="size-4 shrink-0 text-muted-foreground" />
                <span className="truncate">{attachment.filename}</span>
            </TableCell>
            <TableCell>
                <Badge>{attachmentTypeLabels[attachment.type] ?? attachment.type ?? "—"}</Badge>
            </TableCell>
            <TableCell className="uppercase text-muted-foreground">{attachment.extension}</TableCell>
            <TableCell className="tabular-nums">{formatFileSize(attachment.size)}</TableCell>
            <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDate(attachment.createdAt)}
            </TableCell>
            <TableCell>
                {attachment.url ? (
                    <a
                        href={attachment.url}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-sm underline underline-offset-2 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                    >
                        Abrir
                        <ExternalLinkIcon className="size-3.5" />
                    </a>
                ) : (
                    <span className="text-muted-foreground">—</span>
                )}
            </TableCell>
            {managing ? (
                <TableCell>
                    <div className="flex items-center justify-end gap-1">
                        {confirmingDelete ? (
                            <>
                                <span className="text-xs text-muted-foreground">¿Eliminar?</span>
                                <Button
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setConfirmingDelete(false)}
                                    disabled={deleteMutation.isPending}
                                >
                                    No
                                </Button>
                                <Button
                                    variant="destructive"
                                    size="xs"
                                    onClick={() => deleteMutation.mutate(attachment.id)}
                                    disabled={deleteMutation.isPending}
                                >
                                    {deleteMutation.isPending ? "Eliminando…" : "Sí"}
                                </Button>
                            </>
                        ) : (
                            <>
                                <ReplaceAttachmentDialog productId={productId} attachment={attachment} />
                                <Button
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setConfirmingDelete(true)}
                                >
                                    <Trash2Icon />
                                    Eliminar
                                </Button>
                            </>
                        )}
                    </div>
                </TableCell>
            ) : null}
        </TableRow>
    )
}

/**
 * Card "Archivos adjuntos" del detalle de producto: tabla de solo lectura + un
 * modo "Editar" que habilita agregar archivos por URL (con su tipo), reemplazar
 * y eliminar. Como los adjuntos no tienen "principal", no hay selección de
 * portada.
 */
export function ProductAttachmentsCard({ attachments }: { attachments: ProductMediaItem[] }) {
    const { productId } = useParams<{ productId: string }>()
    const [managing, setManaging] = useState(false)

    return (
        <SectionCard
            title="Archivos adjuntos"
            description="Manuales, fichas técnicas y certificados asociados al producto."
            action={
                <div className="flex items-center gap-1.5">
                    {managing && attachments.length > 0 ? (
                        <AddAttachmentsDialog productId={productId} />
                    ) : null}
                    <Button variant="ghost" size="sm" onClick={() => setManaging((value) => !value)}>
                        {managing ? (
                            "Listo"
                        ) : (
                            <>
                                <PencilIcon />
                                Editar
                            </>
                        )}
                    </Button>
                </div>
            }
        >
            {attachments.length === 0 ? (
                managing ? (
                    <div className="flex flex-col items-center gap-3 py-6">
                        <p className="text-sm text-muted-foreground">Este producto no tiene archivos adjuntos.</p>
                        <AddAttachmentsDialog productId={productId} triggerVariant="outline" />
                    </div>
                ) : (
                    <SectionState state="empty" message="Este producto no tiene archivos adjuntos." />
                )
            ) : (
                <Table containerClassName="overflow-x-auto">
                    <TableHeader>
                        <TableRow>
                            <TableHead>Archivo</TableHead>
                            <TableHead>Tipo</TableHead>
                            <TableHead>Formato</TableHead>
                            <TableHead>Tamaño</TableHead>
                            <TableHead>Agregado</TableHead>
                            <TableHead>Enlace</TableHead>
                            {managing ? <TableHead /> : null}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {attachments.map((attachment) => (
                            <AttachmentRow
                                key={`${attachment.id}-${attachment.fileId}`}
                                productId={productId}
                                attachment={attachment}
                                managing={managing}
                            />
                        ))}
                    </TableBody>
                </Table>
            )}
        </SectionCard>
    )
}

// --- Videos ------------------------------------------------------------------

function AddVideosDialog({
    productId,
    triggerVariant = "default",
}: {
    productId?: string
    triggerVariant?: "default" | "outline"
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [text, setText] = useState("")
    const [results, setResults] = useState<ProductVideoImportResult[] | null>(null)
    const [error, setError] = useState<string | null>(null)

    const mutation = useAddProductVideos(productId)
    const urls = useMemo(() => parseUrls(text), [text])

    function reset() {
        setText("")
        setResults(null)
        setError(null)
        mutation.reset()
    }

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            reset()
        }
    }

    async function submit() {
        setError(null)
        setResults(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        if (urls.length === 0) {
            setError("Agregá al menos una URL.")
            return
        }
        try {
            const response = await mutation.mutateAsync({ storageDiskId: diskId, urls })
            setResults(response.results)
            if (response.results.every((r) => r.success)) {
                handleOpenChange(false)
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudieron agregar los videos."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant={triggerVariant} size="sm">
                        <VideoIcon />
                        Agregar videos
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-xl max-w-[calc(100%-2rem)] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Agregar videos por URL</DialogTitle>
                    <DialogDescription>
                        Pegá una URL por línea. Cada URL se valida (debe responder) antes de asociarla al
                        producto. Se toma la URL donde ya está alojado el archivo, no se sube.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />

                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            URLs
                        </label>
                        <Textarea
                            value={text}
                            onChange={(event) => {
                                setText(event.target.value)
                                setResults(null)
                            }}
                            rows={5}
                            placeholder={"https://cdn.ejemplo.com/video-1.mp4\nhttps://cdn.ejemplo.com/video-2.mp4"}
                            disabled={mutation.isPending}
                            aria-label="URLs de los videos"
                        />
                        {urls.length > 0 ? (
                            <p className="text-xs text-muted-foreground">
                                {urls.length} URL{urls.length === 1 ? "" : "s"} detectada
                                {urls.length === 1 ? "" : "s"}
                            </p>
                        ) : null}
                    </div>

                    {results ? (
                        <ul className="flex flex-col gap-1.5 rounded-md border border-border bg-muted/40 p-3">
                            {results.map((result, index) => (
                                <li key={`${result.url}-${index}`} className="flex items-start gap-2 text-xs">
                                    {result.success ? (
                                        <CheckIcon className="mt-0.5 size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                                    ) : (
                                        <XIcon className="mt-0.5 size-3.5 shrink-0 text-destructive" />
                                    )}
                                    <span className="min-w-0 break-all">
                                        <span className="text-muted-foreground">{result.url}</span>
                                        {result.error ? (
                                            <span className="text-destructive"> — {result.error}</span>
                                        ) : null}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    ) : null}

                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cerrar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending || urls.length === 0}>
                        {mutation.isPending ? "Agregando…" : "Agregar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function ReplaceVideoDialog({
    productId,
    video,
}: {
    productId?: string
    video: ProductMediaItem
}) {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [url, setUrl] = useState("")
    const [error, setError] = useState<string | null>(null)

    const mutation = useReplaceProductVideo(productId)

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (!next) {
            setUrl("")
            setError(null)
            mutation.reset()
        }
    }

    async function submit() {
        setError(null)
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        const trimmed = url.trim()
        if (!trimmed) {
            setError("Ingresá la URL del nuevo video.")
            return
        }
        try {
            const result = await mutation.mutateAsync({ videoId: video.id, storageDiskId: diskId, url: trimmed })
            if (result.success) {
                handleOpenChange(false)
            } else {
                setError(result.error ?? "No se pudo reemplazar el video.")
            }
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo reemplazar el video."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant="ghost" size="xs" aria-label="Reemplazar video">
                        <RefreshCwIcon />
                        Reemplazar
                    </Button>
                }
            />
            <DialogContent className="w-120 max-w-[calc(100%-2rem)]">
                <DialogHeader>
                    <DialogTitle>Reemplazar video</DialogTitle>
                    <DialogDescription>El video actual se elimina y se asocia la nueva URL.</DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <StorageDiskSelect value={diskId} onChange={setDiskId} disabled={mutation.isPending} />
                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Nueva URL
                        </label>
                        <Input
                            value={url}
                            onChange={(event) => setUrl(event.target.value)}
                            placeholder="https://cdn.ejemplo.com/nuevo.mp4"
                            disabled={mutation.isPending}
                            aria-label="Nueva URL del video"
                        />
                    </div>
                    {error ? <p className="text-xs text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm" disabled={mutation.isPending}>Cancelar</Button>} />
                    <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Reemplazando…" : "Reemplazar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function VideoRow({
    productId,
    video,
    managing,
}: {
    productId?: string
    video: ProductMediaItem
    managing: boolean
}) {
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    const deleteMutation = useDeleteProductVideo(productId)

    return (
        <TableRow>
            <TableCell className="flex items-center gap-2">
                <VideoIcon className="size-4 shrink-0 text-muted-foreground" />
                <span className="truncate">{video.filename}</span>
            </TableCell>
            <TableCell className="uppercase text-muted-foreground">{video.extension}</TableCell>
            <TableCell className="tabular-nums">{formatFileSize(video.size)}</TableCell>
            <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDate(video.createdAt)}
            </TableCell>
            <TableCell>
                {video.url ? (
                    <a
                        href={video.url}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-sm underline underline-offset-2 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                    >
                        Abrir
                        <ExternalLinkIcon className="size-3.5" />
                    </a>
                ) : (
                    <span className="text-muted-foreground">—</span>
                )}
            </TableCell>
            {managing ? (
                <TableCell>
                    <div className="flex items-center justify-end gap-1">
                        {confirmingDelete ? (
                            <>
                                <span className="text-xs text-muted-foreground">¿Eliminar?</span>
                                <Button
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setConfirmingDelete(false)}
                                    disabled={deleteMutation.isPending}
                                >
                                    No
                                </Button>
                                <Button
                                    variant="destructive"
                                    size="xs"
                                    onClick={() => deleteMutation.mutate(video.id)}
                                    disabled={deleteMutation.isPending}
                                >
                                    {deleteMutation.isPending ? "Eliminando…" : "Sí"}
                                </Button>
                            </>
                        ) : (
                            <>
                                <ReplaceVideoDialog productId={productId} video={video} />
                                <Button variant="ghost" size="xs" onClick={() => setConfirmingDelete(true)}>
                                    <Trash2Icon />
                                    Eliminar
                                </Button>
                            </>
                        )}
                    </div>
                </TableCell>
            ) : null}
        </TableRow>
    )
}

/**
 * Card "Videos" del detalle de producto: tabla de solo lectura + un modo
 * "Editar" que habilita agregar videos por URL, reemplazar y eliminar. El caso
 * más simple — sin orden, portada ni tipo.
 */
export function ProductVideosCard({ videos }: { videos: ProductMediaItem[] }) {
    const { productId } = useParams<{ productId: string }>()
    const [managing, setManaging] = useState(false)

    return (
        <SectionCard
            title="Videos"
            action={
                <div className="flex items-center gap-1.5">
                    {managing && videos.length > 0 ? <AddVideosDialog productId={productId} /> : null}
                    <Button variant="ghost" size="sm" onClick={() => setManaging((value) => !value)}>
                        {managing ? (
                            "Listo"
                        ) : (
                            <>
                                <PencilIcon />
                                Editar
                            </>
                        )}
                    </Button>
                </div>
            }
        >
            {videos.length === 0 ? (
                managing ? (
                    <div className="flex flex-col items-center gap-3 py-6">
                        <p className="text-sm text-muted-foreground">Este producto no tiene videos.</p>
                        <AddVideosDialog productId={productId} triggerVariant="outline" />
                    </div>
                ) : (
                    <SectionState state="empty" message="Este producto no tiene videos." />
                )
            ) : (
                <Table containerClassName="overflow-x-auto">
                    <TableHeader>
                        <TableRow>
                            <TableHead>Archivo</TableHead>
                            <TableHead>Formato</TableHead>
                            <TableHead>Tamaño</TableHead>
                            <TableHead>Agregado</TableHead>
                            <TableHead>Enlace</TableHead>
                            {managing ? <TableHead /> : null}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {videos.map((video) => (
                            <VideoRow
                                key={`${video.id}-${video.fileId}`}
                                productId={productId}
                                video={video}
                                managing={managing}
                            />
                        ))}
                    </TableBody>
                </Table>
            )}
        </SectionCard>
    )
}

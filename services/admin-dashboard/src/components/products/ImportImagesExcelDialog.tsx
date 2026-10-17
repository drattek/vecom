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
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { getServerErrorMessage } from "@/lib/api"
import { useStorageDisks } from "@/lib/product-detail"
import {
    parseImagesSheet,
    useImportImagesBySku,
    type ImageImportItemResult,
    type ParsedImagesSheet,
} from "@/lib/product-images-import"
import { FileSpreadsheetIcon, Loader2Icon } from "lucide-react"
import { useMemo, useState } from "react"

// Misma clave que usa Multimedia en el detalle de producto, para compartir el disco elegido.
const DISK_STORAGE_KEY = "product-media-storage-disk"

function readStoredDiskId(): number | null {
    try {
        const parsed = Number(localStorage.getItem(DISK_STORAGE_KEY))
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

export function ImportImagesExcelDialog() {
    const [open, setOpen] = useState(false)
    const [diskId, setDiskId] = useState<number | null>(readStoredDiskId)
    const [fileName, setFileName] = useState<string | null>(null)
    const [parsed, setParsed] = useState<ParsedImagesSheet | null>(null)
    const [parsing, setParsing] = useState(false)
    const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)
    const [results, setResults] = useState<ImageImportItemResult[] | null>(null)
    const [error, setError] = useState<string | null>(null)

    const disks = useStorageDisks()
    const mutation = useImportImagesBySku()

    const totalUrls = useMemo(
        () => parsed?.groups.reduce((sum, group) => sum + group.urls.length, 0) ?? 0,
        [parsed],
    )

    const failures = useMemo(() => (results ?? []).filter((result) => !result.success), [results])

    function reset() {
        setFileName(null)
        setParsed(null)
        setProgress(null)
        setResults(null)
        setError(null)
        mutation.reset()
    }

    function handleOpenChange(next: boolean) {
        if (mutation.isPending) return
        setOpen(next)
        if (!next) reset()
    }

    async function handleFile(file: File | undefined) {
        reset()
        if (!file) return
        setFileName(file.name)
        setParsing(true)
        try {
            const sheet = await parseImagesSheet(file)
            if (sheet.groups.length === 0) {
                setError("El archivo no tiene filas válidas. Columna A: sku, columna B: image_url.")
            }
            setParsed(sheet)
        } catch (readError) {
            setError(getServerErrorMessage(readError, "No se pudo leer el archivo. Verificá que sea un .xlsx válido."))
        } finally {
            setParsing(false)
        }
    }

    async function submit() {
        if (!parsed || parsed.groups.length === 0) return
        if (diskId == null) {
            setError("Elegí un disco de almacenamiento.")
            return
        }
        setError(null)
        setResults(null)
        setProgress({ done: 0, total: parsed.groups.length })
        const outcome = await mutation.mutateAsync({
            storageDiskId: diskId,
            groups: parsed.groups,
            onProgress: (done, total) => setProgress({ done, total }),
        })
        setResults(outcome)
    }

    const diskItems = (disks.data ?? []).map((disk) => ({ label: disk.name, value: String(disk.id) }))
    const succeeded = (results?.length ?? 0) - failures.length

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button size="sm">
                        <FileSpreadsheetIcon />
                        Importar Excel
                    </Button>
                }
            />
            <DialogContent className="max-h-[85dvh] w-2xl max-w-[calc(100%-2rem)] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Importar imágenes desde Excel</DialogTitle>
                    <DialogDescription>
                        Archivo .xlsx con dos columnas: A = sku, B = image_url. Las URLs se agrupan por SKU
                        (la primera es la portada) y cada una se valida antes de guardarse.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Disco de almacenamiento
                        </label>
                        <Select
                            items={diskItems}
                            value={diskId != null ? String(diskId) : ""}
                            onValueChange={(next) => {
                                const id = Number(next)
                                if (Number.isFinite(id) && id > 0) {
                                    setDiskId(id)
                                    storeDiskId(id)
                                }
                            }}
                            disabled={mutation.isPending || disks.isLoading || diskItems.length === 0}
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
                    </div>

                    <div className="flex flex-col gap-1">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Archivo
                        </label>
                        <input
                            type="file"
                            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                            disabled={mutation.isPending}
                            onChange={(event) => {
                                void handleFile(event.target.files?.[0])
                                event.target.value = ""
                            }}
                            aria-label="Archivo Excel"
                            className="text-sm file:mr-3 file:rounded-md file:border file:border-border file:bg-muted file:px-3 file:py-1.5 file:text-sm"
                        />
                        {fileName ? <p className="text-xs text-muted-foreground">{fileName}</p> : null}
                        {parsing ? (
                            <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                                <Loader2Icon className="size-3.5 animate-spin" />
                                Leyendo archivo…
                            </p>
                        ) : null}
                    </div>

                    {parsed && parsed.groups.length > 0 ? (
                        <div className="flex flex-col gap-2">
                            <p className="text-sm">
                                <span className="font-medium">{parsed.groups.length}</span> SKUs ·{" "}
                                <span className="font-medium">{totalUrls}</span> imágenes
                                {parsed.skippedRows > 0 ? (
                                    <span className="text-muted-foreground">
                                        {" "}· {parsed.skippedRows} filas incompletas omitidas
                                    </span>
                                ) : null}
                                {parsed.duplicateUrls > 0 ? (
                                    <span className="text-muted-foreground">
                                        {" "}· {parsed.duplicateUrls} URLs repetidas omitidas
                                    </span>
                                ) : null}
                            </p>
                            <div className="max-h-64 overflow-y-auto rounded-md border border-border">
                                {parsed.groups.map((group) => (
                                    <div key={group.sku} className="border-b border-border px-3 py-2 last:border-b-0">
                                        <p className="text-sm font-medium">
                                            {group.sku}{" "}
                                            <span className="font-normal text-muted-foreground">
                                                ({group.urls.length})
                                            </span>
                                        </p>
                                        <ul className="mt-1 flex flex-col gap-0.5">
                                            {group.urls.map((url) => (
                                                <li key={url} className="truncate text-xs text-muted-foreground" title={url}>
                                                    {url}
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                ))}
                            </div>
                        </div>
                    ) : null}

                    {mutation.isPending && progress ? (
                        <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                            <Loader2Icon className="size-4 animate-spin" />
                            Subiendo… {progress.done} de {progress.total} SKUs
                        </p>
                    ) : null}

                    {results ? (
                        <div className="flex flex-col gap-2">
                            <p className="text-sm">
                                <span className="font-medium">{succeeded}</span> imágenes guardadas
                                {failures.length > 0 ? (
                                    <span className="text-destructive"> · {failures.length} con error</span>
                                ) : null}
                            </p>
                            {failures.length > 0 ? (
                                <ul className="max-h-48 overflow-y-auto rounded-md border border-destructive/30 p-2 text-xs">
                                    {failures.map((failure, index) => (
                                        <li key={`${failure.sku}-${failure.url}-${index}`} className="py-0.5">
                                            <span className="font-medium">{failure.sku || "—"}</span>
                                            {failure.url ? <span className="text-muted-foreground"> · {failure.url}</span> : null}
                                            <span className="text-destructive"> · {failure.error ?? "Error"}</span>
                                        </li>
                                    ))}
                                </ul>
                            ) : null}
                        </div>
                    ) : null}

                    {error ? <p className="text-sm text-destructive">{error}</p> : null}
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={mutation.isPending}>
                        {results ? "Cerrar" : "Cancelar"}
                    </Button>
                    {!results ? (
                        <Button
                            onClick={() => void submit()}
                            disabled={mutation.isPending || parsing || !parsed || parsed.groups.length === 0}
                        >
                            {mutation.isPending ? "Subiendo…" : "Subir imágenes"}
                        </Button>
                    ) : null}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

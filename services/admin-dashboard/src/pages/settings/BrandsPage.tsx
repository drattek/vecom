import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { TablePagination } from "@/components/TablePagination"
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { apiClient, getServerErrorMessage } from "@/lib/api"
import { brandSchema, paginatedBrandsResponseSchema, type Brand } from "@/lib/schemas/brands"
import { useBrandsPaginationStore } from "@/stores/brandsPaginationStore"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

function useInvalidateBrands() {
    const queryClient = useQueryClient()
    return () => {
        queryClient.invalidateQueries({ queryKey: ["brands"] })
        queryClient.invalidateQueries({ queryKey: ["brand-options"] })
    }
}

/** Alta o edición de una marca (POST / PUT /api/brands). */
function useSaveBrand() {
    const invalidate = useInvalidateBrands()
    return useMutation({
        mutationFn: async (input: { id?: number; name: string }) => {
            const response = input.id
                ? await apiClient.put(`/api/brands/${input.id}`, { name: input.name })
                : await apiClient.post("/api/brands", { name: input.name })
            return brandSchema.parse(response.data)
        },
        onSuccess: invalidate,
    })
}

/** Baja de una marca; el backend responde 409 mientras algo la use. */
function useDeleteBrand() {
    const invalidate = useInvalidateBrands()
    return useMutation({
        mutationFn: async (id: number) => {
            await apiClient.delete(`/api/brands/${id}`)
        },
        onSuccess: invalidate,
    })
}

function BrandDialog({
    open,
    onOpenChange,
    item,
    onSaved,
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    /** Marca a editar; `null` para crear. */
    item: Brand | null
    onSaved: (message: string) => void
}) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <BrandForm item={item} onClose={() => onOpenChange(false)} onSaved={onSaved} />
            </DialogContent>
        </Dialog>
    )
}

function BrandForm({
    item,
    onClose,
    onSaved,
}: {
    item: Brand | null
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const [name, setName] = useState(item?.name ?? "")
    const [error, setError] = useState<string | null>(null)
    const mutation = useSaveBrand()

    async function submit() {
        const trimmed = name.trim()
        if (!trimmed) {
            setError("El nombre es obligatorio.")
            return
        }
        setError(null)

        try {
            await mutation.mutateAsync({ id: item?.id, name: trimmed })
            onSaved(item ? "Marca actualizada." : "Marca creada.")
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar la marca."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{item ? "Editar marca" : "Nueva marca"}</DialogTitle>
                <DialogDescription>
                    {item
                        ? "Cambiar el nombre afecta a todos los productos y vehículos que usan esta marca."
                        : "Los nombres de marca no se repiten (sin distinguir mayúsculas)."}
                </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-1.5">
                <Label htmlFor="brand-name">Nombre</Label>
                <Input
                    id="brand-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    disabled={mutation.isPending}
                    aria-invalid={error ? true : undefined}
                    maxLength={255}
                    autoFocus
                    onKeyDown={(event) => {
                        if (event.key === "Enter") {
                            event.preventDefault()
                            void submit()
                        }
                    }}
                />
                {error ? (
                    <p className="text-xs text-destructive" role="alert">
                        {error}
                    </p>
                ) : null}
            </div>

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={mutation.isPending}>
                    Cancelar
                </Button>
                <Button size="sm" onClick={() => void submit()} disabled={mutation.isPending}>
                    {mutation.isPending ? "Guardando…" : item ? "Guardar" : "Crear"}
                </Button>
            </DialogFooter>
        </>
    )
}

export function BrandsPage() {
    const offset = useBrandsPaginationStore((state) => state.offset)
    const pageSize = useBrandsPaginationStore((state) => state.pageSize)
    const setOffset = useBrandsPaginationStore((state) => state.setOffset)
    const setPageSize = useBrandsPaginationStore((state) => state.setPageSize)

    const [notice, setNotice] = useState<string | null>(null)
    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<Brand | null>(null)
    const [deleting, setDeleting] = useState<Brand | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)
    const deleteMutation = useDeleteBrand()

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["brands", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/brands", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedBrandsResponseSchema.parse(response.data);
        }
    })

    const brands = data?.brands ?? []
    const total = data?.total ?? 0

    function openEditor(brand: Brand | null) {
        setEditing(brand)
        setEditorOpen(true)
    }

    async function confirmDelete() {
        if (!deleting) {
            return
        }
        setDeleteError(null)
        try {
            await deleteMutation.mutateAsync(deleting.id)
            setNotice("Marca eliminada.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar la marca."))
        }
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Marcas</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Marcas disponibles en el catálogo de productos. Una marca en uso por productos, vehículos u otros
                        registros no se puede eliminar.
                    </p>
                </div>
                <Button size="sm" onClick={() => openEditor(null)}>
                    <PlusIcon />
                    Nueva marca
                </Button>
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las marcas: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Productos</TableHead>
                        <TableHead>Vehículos</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && brands.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                Cargando marcas...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && brands.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                No se encontraron marcas.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {brands.map((brand) => (
                        <TableRow key={brand.id}>
                            <TableCell>{brand.name}</TableCell>
                            <TableCell className={brand.productCount === 0 ? "text-muted-foreground" : ""}>
                                {brand.productCount}
                            </TableCell>
                            <TableCell className={brand.vehicleFitmentCount === 0 ? "text-muted-foreground" : ""}>
                                {brand.vehicleFitmentCount}
                            </TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Editar ${brand.name}`}
                                        onClick={() => openEditor(brand)}
                                    >
                                        <PencilIcon />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Eliminar ${brand.name}`}
                                        onClick={() => {
                                            setDeleteError(null)
                                            setDeleting(brand)
                                        }}
                                    >
                                        <Trash2Icon />
                                    </Button>
                                </div>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={total}
                onOffsetChange={setOffset}
                onPageSizeChange={setPageSize}
            />

            <BrandDialog open={editorOpen} onOpenChange={setEditorOpen} item={editing} onSaved={setNotice} />

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar marca"
                description={
                    deleting ? (
                        <>
                            Se eliminará la marca <strong>{deleting.name}</strong>.
                        </>
                    ) : null
                }
                pending={deleteMutation.isPending}
                error={deleteError}
                onConfirm={() => void confirmDelete()}
            />
        </section>
    )
}

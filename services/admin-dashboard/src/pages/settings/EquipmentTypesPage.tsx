import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { EquipmentTypeDialog } from "@/components/fitments/FitmentDialogs"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { useDeleteEquipmentType, useEquipmentTypes } from "@/lib/fitments"
import { formatDate } from "@/lib/product-detail"
import type { EquipmentType } from "@/lib/schemas/fitments"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

export function EquipmentTypesPage() {
    const { data, isLoading, isError, error } = useEquipmentTypes()
    const deleteMutation = useDeleteEquipmentType()

    const [notice, setNotice] = useState<string | null>(null)
    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<EquipmentType | null>(null)
    const [deleting, setDeleting] = useState<EquipmentType | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    const types = data ?? []

    function openEditor(type: EquipmentType | null) {
        setEditing(type)
        setEditorOpen(true)
    }

    async function confirmDelete() {
        if (!deleting) {
            return
        }
        setDeleteError(null)
        try {
            await deleteMutation.mutateAsync(deleting.id)
            setNotice("Tipo eliminado.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar el tipo."))
        }
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Tipos de maquinaria</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Clasificación de la maquinaria del catálogo. Un tipo en uso por alguna maquinaria no se puede eliminar.
                    </p>
                </div>
                <Button size="sm" onClick={() => openEditor(null)}>
                    <PlusIcon />
                    Nuevo tipo
                </Button>
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los tipos: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Maquinaria</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                Cargando tipos...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && types.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center text-muted-foreground">
                                Aún no hay tipos. Crea el primero con «Nuevo tipo».
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {types.map((type) => (
                        <TableRow key={type.id}>
                            <TableCell>{type.name}</TableCell>
                            <TableCell className={type.fitmentCount === 0 ? "text-muted-foreground" : ""}>
                                {type.fitmentCount}
                            </TableCell>
                            <TableCell className="whitespace-nowrap text-muted-foreground">
                                {formatDate(type.updatedAt, true)}
                            </TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Editar ${type.name}`}
                                        onClick={() => openEditor(type)}
                                    >
                                        <PencilIcon />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={`Eliminar ${type.name}`}
                                        title={
                                            type.fitmentCount > 0
                                                ? `En uso por ${type.fitmentCount} maquinaria(s): no se puede eliminar`
                                                : undefined
                                        }
                                        disabled={type.fitmentCount > 0}
                                        onClick={() => {
                                            setDeleteError(null)
                                            setDeleting(type)
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

            <EquipmentTypeDialog open={editorOpen} onOpenChange={setEditorOpen} item={editing} onSaved={setNotice} />

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar tipo de maquinaria"
                description={
                    deleting ? (
                        <>
                            Se eliminará el tipo <strong>{deleting.name}</strong>.
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

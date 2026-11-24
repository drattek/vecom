import { useAuth } from "@/auth/AuthContext"
import { ConfirmDeleteDialog } from "@/components/fitments/ConfirmDeleteDialog"
import { RoleDialog } from "@/components/rbac/RoleDialog"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { useDeleteRole, useRoles } from "@/lib/rbac"
import type { Role } from "@/lib/schemas/rbac"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

export function RolesPage() {
    const { can } = useAuth()
    const { data, isLoading, isError, error } = useRoles()
    const deleteMutation = useDeleteRole()

    const [notice, setNotice] = useState<string | null>(null)
    const [editorOpen, setEditorOpen] = useState(false)
    const [editing, setEditing] = useState<Role | null>(null)
    const [deleting, setDeleting] = useState<Role | null>(null)
    const [deleteError, setDeleteError] = useState<string | null>(null)

    const roles = data ?? []

    function openEditor(role: Role | null) {
        setEditing(role)
        setEditorOpen(true)
    }

    async function confirmDelete() {
        if (!deleting) {
            return
        }
        setDeleteError(null)
        try {
            await deleteMutation.mutateAsync(deleting.id)
            setNotice("Rol eliminado.")
            setDeleting(null)
        } catch (submitError) {
            setDeleteError(getServerErrorMessage(submitError, "No se pudo eliminar el rol."))
        }
    }

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h2 className="text-xl font-semibold text-foreground">Roles</h2>
                    <p className="mt-2 text-sm text-muted-foreground">
                        Un rol agrupa los permisos de una función. Los administradores tienen acceso total sin necesitar rol. Un rol
                        con usuarios asignados no se puede eliminar.
                    </p>
                </div>
                {can("roles.create") ? (
                    <Button size="sm" onClick={() => openEditor(null)}>
                        <PlusIcon />
                        Nuevo rol
                    </Button>
                ) : null}
            </div>

            {notice ? (
                <p className="mt-3 text-xs text-muted-foreground" role="status">
                    {notice}
                </p>
            ) : null}
            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los roles: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Rol</TableHead>
                        <TableHead>Descripción</TableHead>
                        <TableHead>Permisos</TableHead>
                        <TableHead>Usuarios</TableHead>
                        <TableHead className="w-24">Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                Cargando roles...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && roles.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                Aún no hay roles. Crea el primero con «Nuevo rol».
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {roles.map((role) => (
                        <TableRow key={role.id}>
                            <TableCell className="font-medium">{role.name}</TableCell>
                            <TableCell className="text-muted-foreground">{role.description || "-"}</TableCell>
                            <TableCell>{role.permissionCount}</TableCell>
                            <TableCell className={role.userCount === 0 ? "text-muted-foreground" : ""}>{role.userCount}</TableCell>
                            <TableCell>
                                <div className="flex items-center gap-1">
                                    {can("roles.edit") ? (
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            aria-label={`Editar ${role.name}`}
                                            onClick={() => openEditor(role)}
                                        >
                                            <PencilIcon />
                                        </Button>
                                    ) : null}
                                    {can("roles.delete") ? (
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            aria-label={`Eliminar ${role.name}`}
                                            title={
                                                role.userCount > 0
                                                    ? `Tiene ${role.userCount} usuario(s) asignado(s): no se puede eliminar`
                                                    : undefined
                                            }
                                            disabled={role.userCount > 0}
                                            onClick={() => {
                                                setDeleteError(null)
                                                setDeleting(role)
                                            }}
                                        >
                                            <Trash2Icon />
                                        </Button>
                                    ) : null}
                                </div>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

            <RoleDialog open={editorOpen} onOpenChange={setEditorOpen} role={editing} onSaved={setNotice} />

            <ConfirmDeleteDialog
                open={deleting !== null}
                onOpenChange={(open) => {
                    if (!open && !deleteMutation.isPending) {
                        setDeleting(null)
                    }
                }}
                title="Eliminar rol"
                description={
                    deleting ? (
                        <>
                            Se eliminará el rol <strong>{deleting.name}</strong>. Esta acción no se puede deshacer.
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

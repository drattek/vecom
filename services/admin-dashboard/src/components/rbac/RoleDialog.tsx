import { RolePermissionPicker } from "@/components/rbac/PermissionPickers"
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
import { Textarea } from "@/components/ui/textarea"
import { getServerErrorMessage } from "@/lib/api"
import { usePermissionsCatalog, useRole, useSaveRole } from "@/lib/rbac"
import type { Role } from "@/lib/schemas/rbac"
import { useState } from "react"

type RoleDialogProps = {
    open: boolean
    onOpenChange: (open: boolean) => void
    /** Rol a editar; null = crear uno nuevo. */
    role: Role | null
    onSaved: (message: string) => void
}

/** Alta / edición de un rol con su matriz de permisos. */
export function RoleDialog({ open, onOpenChange, role, onSaved }: RoleDialogProps) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl">
                {open ? <RoleDialogBody role={role} onClose={() => onOpenChange(false)} onSaved={onSaved} /> : null}
            </DialogContent>
        </Dialog>
    )
}

/** Carga el catálogo (y el rol al editar) y monta el formulario ya con sus datos. */
function RoleDialogBody({ role, onClose, onSaved }: { role: Role | null; onClose: () => void; onSaved: (message: string) => void }) {
    const catalog = usePermissionsCatalog()
    const detail = useRole(role ? role.id : null)

    const loading = catalog.isLoading || (role !== null && detail.isLoading)
    const failed = catalog.isError || (role !== null && detail.isError)

    if (loading || failed || !catalog.data) {
        return (
            <>
                <DialogHeader>
                    <DialogTitle>{role ? "Editar rol" : "Nuevo rol"}</DialogTitle>
                    <DialogDescription>{failed ? "No se pudo cargar la información del rol." : "Cargando permisos…"}</DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="ghost" size="sm" onClick={onClose}>
                        Cerrar
                    </Button>
                </DialogFooter>
            </>
        )
    }

    return <RoleForm role={role} initial={detail.data ?? null} permissions={catalog.data} onClose={onClose} onSaved={onSaved} />
}

function RoleForm({
    role,
    initial,
    permissions,
    onClose,
    onSaved,
}: {
    role: Role | null
    initial: Role | null
    permissions: NonNullable<ReturnType<typeof usePermissionsCatalog>["data"]>
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const mutation = useSaveRole()

    const [name, setName] = useState(initial?.name ?? "")
    const [description, setDescription] = useState(initial?.description ?? "")
    const [selected, setSelected] = useState<Set<number>>(() => new Set(initial?.permissionIds ?? []))
    const [error, setError] = useState<string | null>(null)

    async function submit() {
        if (name.trim() === "") {
            setError("El nombre del rol es obligatorio.")
            return
        }
        setError(null)
        try {
            await mutation.mutateAsync({
                id: role?.id,
                name: name.trim(),
                description: description.trim(),
                permissionIds: [...selected],
            })
            onSaved(role ? "Rol actualizado." : "Rol creado.")
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo guardar el rol."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>{role ? "Editar rol" : "Nuevo rol"}</DialogTitle>
                <DialogDescription>
                    Define a qué secciones y acciones da acceso este rol. Los administradores siempre tienen acceso total y no
                    necesitan un rol.
                </DialogDescription>
            </DialogHeader>

            <div className="flex min-h-0 flex-1 flex-col gap-4">
                <div className="grid gap-3 sm:grid-cols-2">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="role-name">Nombre</Label>
                        <Input
                            id="role-name"
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                            disabled={mutation.isPending}
                            maxLength={100}
                            autoFocus
                        />
                    </div>
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="role-description">Descripción</Label>
                        <Textarea
                            id="role-description"
                            className="min-h-8"
                            rows={1}
                            value={description}
                            onChange={(event) => setDescription(event.target.value)}
                            disabled={mutation.isPending}
                            maxLength={255}
                        />
                    </div>
                </div>

                <div className="flex min-h-0 flex-1 flex-col gap-1.5">
                    <Label>Permisos ({selected.size} seleccionados)</Label>
                    <div className="min-h-0 flex-1 overflow-y-auto pr-1">
                        <RolePermissionPicker
                            permissions={permissions}
                            selected={selected}
                            onChange={setSelected}
                            disabled={mutation.isPending}
                        />
                    </div>
                </div>

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
                <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                    {mutation.isPending ? "Guardando…" : "Guardar"}
                </Button>
            </DialogFooter>
        </>
    )
}

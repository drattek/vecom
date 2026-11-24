import { useAuth } from "@/auth/AuthContext"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { UserAccessFields, type UserAccessValue } from "@/components/users/UserAccessFields"
import { getServerErrorMessage } from "@/lib/api"
import { useUpdateUserAccess, useUserDetail } from "@/lib/rbac"
import type { UserDetail } from "@/lib/schemas/rbac"
import type { User } from "@/lib/schemas/users"
import { useState } from "react"

type EditUserDialogProps = {
    open: boolean
    onOpenChange: (open: boolean) => void
    user: User | null
    onSaved: (message: string) => void
}

/** Edita el acceso de un usuario: tipo de cuenta, rol, permisos extra y estado. */
export function EditUserDialog({ open, onOpenChange, user, onSaved }: EditUserDialogProps) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl">
                {open && user ? (
                    <EditUserBody user={user} onClose={() => onOpenChange(false)} onSaved={onSaved} />
                ) : null}
            </DialogContent>
        </Dialog>
    )
}

function EditUserBody({ user, onClose, onSaved }: { user: User; onClose: () => void; onSaved: (message: string) => void }) {
    const detail = useUserDetail(user.id)

    if (detail.isLoading || detail.isError || !detail.data) {
        return (
            <>
                <DialogHeader>
                    <DialogTitle>Acceso de {user.username}</DialogTitle>
                    <DialogDescription>
                        {detail.isError ? "No se pudo cargar el usuario." : "Cargando usuario…"}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="ghost" size="sm" onClick={onClose}>
                        Cerrar
                    </Button>
                </DialogFooter>
            </>
        )
    }

    return <EditUserForm user={user} detail={detail.data} onClose={onClose} onSaved={onSaved} />
}

function EditUserForm({
    user,
    detail,
    onClose,
    onSaved,
}: {
    user: User
    detail: UserDetail
    onClose: () => void
    onSaved: (message: string) => void
}) {
    const { session, refreshUser } = useAuth()
    const mutation = useUpdateUserAccess()

    const [access, setAccess] = useState<UserAccessValue>({
        isAdmin: detail.role === "admin",
        roleId: detail.roleId,
        overrides: detail.overrides,
    })
    const [isActive, setIsActive] = useState(detail.isActive)
    const [error, setError] = useState<string | null>(null)

    const isSelf = session?.user.id === user.id

    async function submit() {
        setError(null)
        try {
            await mutation.mutateAsync({
                id: user.id,
                isAdmin: access.isAdmin,
                roleId: access.isAdmin ? null : access.roleId,
                isActive,
                overrides: access.isAdmin ? [] : access.overrides,
            })
            if (isSelf) {
                await refreshUser().catch(() => undefined)
            }
            onSaved(`Acceso de ${user.username} actualizado.`)
            onClose()
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo actualizar el usuario."))
        }
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>Acceso de {user.username}</DialogTitle>
                <DialogDescription>
                    Los cambios aplican de inmediato: en la siguiente acción del usuario ya rigen los nuevos permisos.
                </DialogDescription>
            </DialogHeader>

            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pr-1">
                <UserAccessFields idPrefix="edit-user" value={access} onChange={setAccess} disabled={mutation.isPending} />

                <div className="flex items-center gap-2">
                    <input
                        id="edit-user-active"
                        type="checkbox"
                        className="size-4 accent-primary"
                        checked={isActive}
                        disabled={mutation.isPending || isSelf}
                        onChange={(event) => setIsActive(event.target.checked)}
                    />
                    <Label htmlFor="edit-user-active">
                        Cuenta activa{isSelf ? " (no puedes desactivar tu propia cuenta)" : ""}
                    </Label>
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

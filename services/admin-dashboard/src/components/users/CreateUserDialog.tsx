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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getServerErrorMessage } from "@/lib/api"
import { useCreateUser } from "@/lib/users"
import { PlusIcon } from "lucide-react"
import { useState } from "react"

const MIN_PASSWORD_LENGTH = 8

export function CreateUserDialog() {
    const [open, setOpen] = useState(false)
    const [username, setUsername] = useState("")
    const [password, setPassword] = useState("")
    const [confirmPassword, setConfirmPassword] = useState("")
    const [error, setError] = useState<string | null>(null)
    const mutation = useCreateUser()

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (next) {
            setUsername("")
            setPassword("")
            setConfirmPassword("")
            setError(null)
            mutation.reset()
        }
    }

    async function submit() {
        const trimmedUsername = username.trim()
        if (trimmedUsername === "") {
            setError("El usuario es obligatorio.")
            return
        }
        if (password.length < MIN_PASSWORD_LENGTH) {
            setError(`La contraseña debe tener al menos ${MIN_PASSWORD_LENGTH} caracteres.`)
            return
        }
        if (password !== confirmPassword) {
            setError("Las contraseñas no coinciden.")
            return
        }
        setError(null)
        try {
            await mutation.mutateAsync({ username: trimmedUsername, password })
            setOpen(false)
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo crear el usuario."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button size="sm">
                        <PlusIcon />
                        Nuevo usuario
                    </Button>
                }
            />
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Nuevo usuario</DialogTitle>
                    <DialogDescription>Crea una cuenta de acceso con usuario y contraseña.</DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="new-user-username">Usuario</Label>
                        <Input
                            id="new-user-username"
                            value={username}
                            onChange={(event) => setUsername(event.target.value)}
                            disabled={mutation.isPending}
                            aria-invalid={error ? true : undefined}
                            autoFocus
                            maxLength={100}
                        />
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="new-user-password">Contraseña</Label>
                        <Input
                            id="new-user-password"
                            type="password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                            disabled={mutation.isPending}
                            aria-invalid={error ? true : undefined}
                        />
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="new-user-confirm-password">Confirmar contraseña</Label>
                        <Input
                            id="new-user-confirm-password"
                            type="password"
                            value={confirmPassword}
                            onChange={(event) => setConfirmPassword(event.target.value)}
                            disabled={mutation.isPending}
                            aria-invalid={error ? true : undefined}
                        />
                    </div>

                    {error ? (
                        <p className="text-xs text-destructive" role="alert">
                            {error}
                        </p>
                    ) : null}
                </div>

                <DialogFooter>
                    <Button variant="ghost" size="sm" onClick={() => setOpen(false)} disabled={mutation.isPending}>
                        Cancelar
                    </Button>
                    <Button size="sm" onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Creando…" : "Crear"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

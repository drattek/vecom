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
import { apiClient, getServerErrorMessage } from "@/lib/api"
import type { StorageDisk } from "@/lib/schemas/storage-disk"
import { useMutation } from "@tanstack/react-query"
import { useState } from "react"

/**
 * Carga las credenciales de Azure Blob Storage de un disco (cuenta, llave y
 * contenedor). El backend las guarda cifradas y no las devuelve nunca, así que
 * los campos siempre empiezan vacíos y guardar reemplaza los tres valores.
 */
export function AzureCredentialsDialog({
    disk,
    onOpenChange,
}: {
    /** Disco al que se le cargan las credenciales; null cierra el diálogo. */
    disk: StorageDisk | null
    onOpenChange: (open: boolean) => void
}) {
    return (
        <Dialog open={disk !== null} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                {/* Se monta solo con un disco: al cerrar y abrir otro el formulario
                    arranca limpio, sin arrastrar la llave anterior. */}
                {disk ? <CredentialsForm key={disk.id} disk={disk} onClose={() => onOpenChange(false)} /> : null}
            </DialogContent>
        </Dialog>
    )
}

function CredentialsForm({ disk, onClose }: { disk: StorageDisk; onClose: () => void }) {
    const [accountName, setAccountName] = useState("")
    const [accountKey, setAccountKey] = useState("")
    const [containerName, setContainerName] = useState("")
    const [errorMessage, setErrorMessage] = useState<string | null>(null)
    const [saved, setSaved] = useState(false)

    const mutation = useMutation({
        mutationFn: async () => {
            await apiClient.put(`/api/storage-disks/${disk.id}/azure-credentials`, {
                accountName: accountName.trim(),
                accountKey: accountKey.trim(),
                containerName: containerName.trim(),
            })
        },
        onSuccess: () => {
            setSaved(true)
            // La llave no debe quedar en memoria del formulario más de lo necesario.
            setAccountKey("")
        },
        onError: (error) => setErrorMessage(getServerErrorMessage(error, "No se pudieron guardar las credenciales.")),
    })

    const canSubmit = accountName.trim() !== "" && accountKey.trim() !== "" && containerName.trim() !== ""

    function submit() {
        if (!canSubmit || mutation.isPending) {
            return
        }
        setErrorMessage(null)
        mutation.mutate()
    }

    return (
        <>
            <DialogHeader>
                <DialogTitle>Credenciales de Azure · {disk.name}</DialogTitle>
                <DialogDescription>
                    Se guardan cifradas y no se pueden volver a consultar. Al guardar se reemplazan las credenciales actuales
                    del disco ({disk.code}); para cambiar de contenedor basta con volver a capturarlas.
                </DialogDescription>
            </DialogHeader>

            {saved ? (
                <p className="text-sm font-medium text-foreground" role="status">
                    Credenciales guardadas.
                </p>
            ) : (
                <form
                    className="flex flex-col gap-3"
                    autoComplete="off"
                    onSubmit={(event) => {
                        event.preventDefault()
                        submit()
                    }}
                >
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="azure-account-name">Nombre de la cuenta</Label>
                        <Input
                            id="azure-account-name"
                            value={accountName}
                            onChange={(event) => setAccountName(event.target.value)}
                            autoComplete="off"
                            autoFocus
                            disabled={mutation.isPending}
                        />
                    </div>
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="azure-account-key">Llave de la cuenta</Label>
                        <Input
                            id="azure-account-key"
                            type="password"
                            value={accountKey}
                            onChange={(event) => setAccountKey(event.target.value)}
                            autoComplete="new-password"
                            disabled={mutation.isPending}
                        />
                    </div>
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="azure-container-name">Nombre del contenedor</Label>
                        <Input
                            id="azure-container-name"
                            value={containerName}
                            onChange={(event) => setContainerName(event.target.value)}
                            autoComplete="off"
                            disabled={mutation.isPending}
                        />
                    </div>

                    {errorMessage ? (
                        <p className="text-sm text-destructive" role="alert">
                            {errorMessage}
                        </p>
                    ) : null}
                </form>
            )}

            <DialogFooter>
                <Button variant="ghost" size="sm" onClick={onClose} disabled={mutation.isPending}>
                    {saved ? "Cerrar" : "Cancelar"}
                </Button>
                {!saved ? (
                    <Button size="sm" onClick={submit} disabled={!canSubmit || mutation.isPending}>
                        {mutation.isPending ? "Guardando…" : "Guardar credenciales"}
                    </Button>
                ) : null}
            </DialogFooter>
        </>
    )
}

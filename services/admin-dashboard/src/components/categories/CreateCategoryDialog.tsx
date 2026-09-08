import { CategoryTreePicker } from "@/components/categories/CategoryTreePicker"
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
import { useCreateCategory } from "@/lib/categories"
import { PlusIcon } from "lucide-react"
import { useState } from "react"

export function CreateCategoryDialog() {
    const [open, setOpen] = useState(false)
    const [name, setName] = useState("")
    const [parentId, setParentId] = useState<number | null>(null)
    const [error, setError] = useState<string | null>(null)
    const mutation = useCreateCategory()

    function handleOpenChange(next: boolean) {
        setOpen(next)
        if (next) {
            setName("")
            setParentId(null)
            setError(null)
            mutation.reset()
        }
    }

    async function submit() {
        const trimmed = name.trim()
        if (trimmed === "") {
            setError("El nombre es obligatorio.")
            return
        }
        setError(null)
        try {
            await mutation.mutateAsync({ name: trimmed, parentId })
            setOpen(false)
        } catch (submitError) {
            setError(getServerErrorMessage(submitError, "No se pudo crear la categoría."))
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button size="sm">
                        <PlusIcon />
                        Nueva categoría
                    </Button>
                }
            />
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Nueva categoría</DialogTitle>
                    <DialogDescription>
                        Crea una categoría local. Elige su categoría padre en el árbol o déjala como raíz.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="new-category-name">Nombre</Label>
                        <Input
                            id="new-category-name"
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                            disabled={mutation.isPending}
                            aria-invalid={error ? true : undefined}
                            autoFocus
                            maxLength={255}
                        />
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label>Categoría padre</Label>
                        <CategoryTreePicker
                            value={parentId}
                            onChange={setParentId}
                            disabled={mutation.isPending}
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

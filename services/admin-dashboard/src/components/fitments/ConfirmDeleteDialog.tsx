import {
    AlertDialog,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"

/**
 * Confirmación de una baja. El diálogo no se cierra solo: lo cierra quien lo
 * usa cuando `onConfirm` termina bien, y si falla muestra `error` en su lugar.
 */
export function ConfirmDeleteDialog({
    open,
    onOpenChange,
    title,
    description,
    confirmLabel = "Eliminar",
    pending,
    error,
    onConfirm,
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    title: string
    description: React.ReactNode
    confirmLabel?: string
    pending: boolean
    error: string | null
    onConfirm: () => void
}) {
    return (
        <AlertDialog open={open} onOpenChange={onOpenChange}>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>{title}</AlertDialogTitle>
                    <AlertDialogDescription>{description}</AlertDialogDescription>
                </AlertDialogHeader>
                {error ? (
                    <p className="text-xs text-destructive" role="alert">
                        {error}
                    </p>
                ) : null}
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>Cancelar</AlertDialogCancel>
                    <Button variant="destructive" onClick={onConfirm} disabled={pending}>
                        {pending ? "Eliminando…" : confirmLabel}
                    </Button>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    )
}

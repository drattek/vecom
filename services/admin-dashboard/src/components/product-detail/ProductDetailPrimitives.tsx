import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { PencilIcon } from "lucide-react"

// Componentes compartidos por las seis secciones del detalle de producto.
// La mayoría son de solo lectura; algunas cards permiten edición inline
// (ver SectionEditActions + PATCH /api/products/{id}/details/*). El hook de
// datos y los formateadores viven en @/lib/product-detail para no romper el
// fast refresh de este archivo.

export function SectionCard({
    title,
    description,
    action,
    className,
    children,
}: {
    title: string
    description?: string
    action?: React.ReactNode
    className?: string
    children: React.ReactNode
}) {
    return (
        <section className={cn("rounded-lg border border-border bg-card", className)}>
            <header className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
                <div className="min-w-0">
                    <h2 className="text-sm font-semibold text-foreground">{title}</h2>
                    {description ? (
                        <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
                    ) : null}
                </div>
                {action}
            </header>
            <div className="p-4">{children}</div>
        </section>
    )
}

/** Fila etiqueta/valor de las listas de definición (General, Atributos). */
export function Field({
    label,
    value,
    className,
}: {
    label: string
    value?: React.ReactNode
    className?: string
}) {
    const isEmpty = value === null || value === undefined || value === ""

    return (
        <div className={cn("min-w-0", className)}>
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</dt>
            <dd className={cn("mt-1 text-sm wrap-break-word", isEmpty ? "text-muted-foreground" : "text-foreground")}>
                {isEmpty ? "—" : value}
            </dd>
        </div>
    )
}

export function Badge({
    children,
    tone = "neutral",
}: {
    children: React.ReactNode
    tone?: "neutral" | "success" | "warning" | "muted"
}) {
    const tones = {
        neutral: "border-border bg-muted text-foreground",
        success: "border-emerald-600/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
        warning: "border-amber-600/30 bg-amber-500/10 text-amber-700 dark:text-amber-500",
        muted: "border-border bg-transparent text-muted-foreground",
    }

    return (
        <span
            className={cn(
                "inline-flex items-center rounded-md border px-1.5 py-0.5 text-xs font-medium whitespace-nowrap",
                tones[tone],
            )}
        >
            {children}
        </span>
    )
}

/**
 * Acciones de edición para la cabecera de un SectionCard (prop `action`):
 * botón "Editar" en reposo, par "Cancelar / Guardar" en modo edición.
 */
export function SectionEditActions({
    isEditing,
    isSaving,
    onEdit,
    onCancel,
    onSave,
}: {
    isEditing: boolean
    isSaving: boolean
    onEdit: () => void
    onCancel: () => void
    onSave: () => void
}) {
    if (!isEditing) {
        return (
            <Button variant="ghost" size="sm" onClick={onEdit}>
                <PencilIcon />
                Editar
            </Button>
        )
    }

    return (
        <div className="flex items-center gap-1.5">
            <Button variant="ghost" size="sm" onClick={onCancel} disabled={isSaving}>
                Cancelar
            </Button>
            <Button size="sm" onClick={onSave} disabled={isSaving}>
                {isSaving ? "Guardando…" : "Guardar"}
            </Button>
        </div>
    )
}

/** Mensaje de error inline para las cards editables (validación / error de guardado). */
export function FieldError({ message }: { message: string | null }) {
    if (!message) {
        return null
    }
    return (
        <p className="mt-1 text-xs text-destructive" role="alert">
            {message}
        </p>
    )
}

export function SectionState({ state, message }: { state: "loading" | "error" | "empty"; message?: string }) {
    const text =
        message ??
        (state === "loading"
            ? "Cargando información..."
            : state === "error"
              ? "No se pudo cargar esta sección."
              : "Sin información para mostrar.")

    return (
        <p className={cn("py-6 text-center text-sm", state === "error" ? "text-destructive" : "text-muted-foreground")}>
            {text}
        </p>
    )
}

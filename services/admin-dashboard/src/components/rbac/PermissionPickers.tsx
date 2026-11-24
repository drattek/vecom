import { actionLabel, groupPermissions } from "@/lib/rbac"
import type { Permission, PermissionOverride } from "@/lib/schemas/rbac"
import { cn } from "@/lib/utils"
import { useMemo } from "react"

// Selectores de permisos agrupados por recurso. Se usan en el editor de roles
// (marcar/desmarcar) y en el acceso de un usuario (excepciones sobre su rol).

type RolePermissionPickerProps = {
    permissions: Permission[]
    selected: Set<number>
    onChange: (next: Set<number>) => void
    disabled?: boolean
}

/** Casillas por recurso/acción para armar los permisos de un rol. */
export function RolePermissionPicker({ permissions, selected, onChange, disabled }: RolePermissionPickerProps) {
    const groups = useMemo(() => groupPermissions(permissions), [permissions])

    function toggle(ids: number[], checked: boolean) {
        const next = new Set(selected)
        for (const id of ids) {
            if (checked) {
                next.add(id)
            } else {
                next.delete(id)
            }
        }
        onChange(next)
    }

    return (
        <div className="flex flex-col gap-2">
            {groups.map((group) => {
                const ids = group.permissions.map((permission) => permission.id)
                const allChecked = ids.every((id) => selected.has(id))
                const someChecked = ids.some((id) => selected.has(id))

                return (
                    <div key={group.resource} className="rounded-lg border border-border p-3">
                        <label className="flex items-center gap-2 text-sm font-medium text-foreground">
                            <input
                                type="checkbox"
                                className="size-4 accent-primary"
                                checked={allChecked}
                                ref={(element) => {
                                    if (element) {
                                        element.indeterminate = someChecked && !allChecked
                                    }
                                }}
                                disabled={disabled}
                                onChange={(event) => toggle(ids, event.target.checked)}
                            />
                            {group.label}
                        </label>
                        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1.5 pl-6">
                            {group.permissions.map((permission) => (
                                <label
                                    key={permission.id}
                                    className="flex items-center gap-1.5 text-sm text-muted-foreground"
                                    title={permission.description}
                                >
                                    <input
                                        type="checkbox"
                                        className="size-4 accent-primary"
                                        checked={selected.has(permission.id)}
                                        disabled={disabled}
                                        onChange={(event) => toggle([permission.id], event.target.checked)}
                                    />
                                    {actionLabel(permission.action)}
                                </label>
                            ))}
                        </div>
                    </div>
                )
            })}
        </div>
    )
}

type UserOverridePickerProps = {
    permissions: Permission[]
    /** Ids de permiso que ya da el rol del usuario. */
    rolePermissionIds: Set<number>
    overrides: PermissionOverride[]
    onChange: (next: PermissionOverride[]) => void
    disabled?: boolean
}

/**
 * Excepciones por usuario sobre los permisos de su rol. Cada chip alterna:
 * lo que da el rol → denegar; lo que no da el rol → conceder; otro clic quita
 * la excepción.
 */
export function UserOverridePicker({ permissions, rolePermissionIds, overrides, onChange, disabled }: UserOverridePickerProps) {
    const groups = useMemo(() => groupPermissions(permissions), [permissions])
    const effectById = useMemo(() => new Map(overrides.map((o) => [o.permissionId, o.effect])), [overrides])

    function click(permissionId: number) {
        const fromRole = rolePermissionIds.has(permissionId)
        const current = effectById.get(permissionId)
        const rest = overrides.filter((o) => o.permissionId !== permissionId)

        if (current) {
            onChange(rest) // quitar la excepción: vuelve a lo que diga el rol
        } else {
            onChange([...rest, { permissionId, effect: fromRole ? "deny" : "grant" }])
        }
    }

    return (
        <div className="flex flex-col gap-2">
            <p className="text-xs text-muted-foreground">
                Los permisos del rol vienen marcados. Haz clic para <span className="text-emerald-600">conceder</span> uno extra
                o <span className="text-destructive">denegar</span> uno del rol; otro clic quita la excepción.
            </p>
            {groups.map((group) => (
                <div key={group.resource} className="rounded-lg border border-border p-3">
                    <p className="text-sm font-medium text-foreground">{group.label}</p>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                        {group.permissions.map((permission) => {
                            const fromRole = rolePermissionIds.has(permission.id)
                            const effect = effectById.get(permission.id)
                            const allowed = effect === "grant" || (fromRole && effect !== "deny")

                            return (
                                <button
                                    key={permission.id}
                                    type="button"
                                    disabled={disabled}
                                    title={permission.description}
                                    onClick={() => click(permission.id)}
                                    className={cn(
                                        "rounded-md border px-2 py-0.5 text-xs transition-colors disabled:opacity-50",
                                        effect === "grant" && "border-emerald-600 bg-emerald-600/10 text-emerald-700 dark:text-emerald-400",
                                        effect === "deny" && "border-destructive bg-destructive/10 text-destructive line-through",
                                        !effect && allowed && "border-primary/40 bg-primary/10 text-foreground",
                                        !effect && !allowed && "border-border text-muted-foreground hover:bg-muted",
                                    )}
                                >
                                    {effect === "grant" ? "+ " : ""}
                                    {actionLabel(permission.action)}
                                </button>
                            )
                        })}
                    </div>
                </div>
            ))}
        </div>
    )
}

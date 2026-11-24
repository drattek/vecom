import { UserOverridePicker } from "@/components/rbac/PermissionPickers"
import { Label } from "@/components/ui/label"
import { usePermissionsCatalog, useRole, useRoles } from "@/lib/rbac"
import type { PermissionOverride } from "@/lib/schemas/rbac"
import { useMemo } from "react"

const SELECT_CLASS =
    "h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"

export type UserAccessValue = {
    isAdmin: boolean
    roleId: number | null
    overrides: PermissionOverride[]
}

type UserAccessFieldsProps = {
    value: UserAccessValue
    onChange: (next: UserAccessValue) => void
    disabled?: boolean
    idPrefix: string
}

/**
 * Tipo de cuenta (administrador / con rol), rol y excepciones de permisos.
 * Compartido por el alta y la edición de usuarios.
 */
export function UserAccessFields({ value, onChange, disabled, idPrefix }: UserAccessFieldsProps) {
    const roles = useRoles()
    const catalog = usePermissionsCatalog()
    const role = useRole(value.isAdmin ? null : value.roleId)

    const rolePermissionIds = useMemo(() => new Set(role.data?.permissionIds ?? []), [role.data])

    return (
        <div className="flex flex-col gap-4">
            <div className="grid gap-3 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor={`${idPrefix}-type`}>Tipo de cuenta</Label>
                    <select
                        id={`${idPrefix}-type`}
                        className={SELECT_CLASS}
                        disabled={disabled}
                        value={value.isAdmin ? "admin" : "role"}
                        onChange={(event) => onChange({ ...value, isAdmin: event.target.value === "admin" })}
                    >
                        <option value="role">Con rol y permisos</option>
                        <option value="admin">Administrador (acceso total)</option>
                    </select>
                </div>

                {!value.isAdmin ? (
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor={`${idPrefix}-role`}>Rol</Label>
                        <select
                            id={`${idPrefix}-role`}
                            className={SELECT_CLASS}
                            disabled={disabled || roles.isLoading}
                            value={value.roleId ?? ""}
                            onChange={(event) =>
                                onChange({ ...value, roleId: event.target.value === "" ? null : Number(event.target.value) })
                            }
                        >
                            <option value="">Sin rol (solo permisos extra)</option>
                            {(roles.data ?? []).map((item) => (
                                <option key={item.id} value={item.id}>
                                    {item.name}
                                </option>
                            ))}
                        </select>
                    </div>
                ) : null}
            </div>

            {value.isAdmin ? (
                <p className="text-xs text-muted-foreground">
                    Los administradores acceden a todo; no se les asignan roles ni permisos.
                </p>
            ) : (
                <div className="flex flex-col gap-1.5">
                    <Label>Permisos extra / denegados</Label>
                    {catalog.isLoading || role.isLoading ? (
                        <p className="text-sm text-muted-foreground">Cargando permisos…</p>
                    ) : catalog.isError ? (
                        <p className="text-sm text-destructive">No se pudo cargar el catálogo de permisos.</p>
                    ) : (
                        <div className="max-h-72 overflow-y-auto pr-1">
                            <UserOverridePicker
                                permissions={catalog.data ?? []}
                                rolePermissionIds={rolePermissionIds}
                                overrides={value.overrides}
                                onChange={(overrides) => onChange({ ...value, overrides })}
                                disabled={disabled}
                            />
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

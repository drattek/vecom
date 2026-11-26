import { apiClient } from "@/lib/api"
import {
    permissionsResponseSchema,
    roleSchema,
    rolesResponseSchema,
    userDetailSchema,
    type Permission,
    type PermissionOverride,
} from "@/lib/schemas/rbac"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

// Hooks de roles y permisos (Configuración → Roles y alta/edición de usuarios).
// Ver ADR 0009.

/** Etiquetas en español de los recursos del catálogo de permisos. */
const RESOURCE_LABELS: Record<string, string> = {
    dashboard: "Dashboard",
    reports: "Reportes de publicaciones",
    sales: "Ventas",
    meli_listings: "Publicaciones de MercadoLibre",
    users: "Usuarios",
    roles: "Roles y permisos",
    products: "Productos",
    products_pending: "Productos pendientes",
    products_channels: "Productos por canal",
    products_images: "Imágenes de productos (herramienta)",
    products_import: "Importación masiva de MercadoLibre",
    products_bulk_update: "Actualización masiva de productos",
    product_pricing: "Detalle: Precios",
    product_inventory: "Detalle: Inventario",
    product_attributes: "Detalle: Atributos",
    product_compatibilities: "Detalle: Compatibilidades",
    product_sync: "Detalle: Sincronización",
    product_media: "Detalle: Multimedia",
    product_part_numbers: "Detalle: Números de parte",
    channel_listings: "Publicaciones en canales",
    mercadolibre: "MercadoLibre",
    storage_disks: "Discos de almacenamiento",
    storage_files: "Archivos almacenados",
    branches: "Sucursales",
    warehouses: "Almacenes",
    currencies: "Divisas",
    exchange_rates: "Tipos de cambio",
    price_lists: "Listas de precios",
    pricing_formulas: "Fórmulas de precios",
    channels: "Canales",
    connections: "Conexiones",
    brands: "Marcas",
    categories: "Categorías",
    vehicle_fitments: "Vehículos (catálogo)",
    equipment_fitments: "Maquinaria (catálogo)",
    equipment_types: "Tipos de maquinaria",
}

const ACTION_LABELS: Record<string, string> = {
    view: "Ver",
    create: "Crear",
    edit: "Editar",
    delete: "Eliminar",
    import: "Importar",
    export: "Exportar",
    retry: "Reintentar",
    publish: "Publicar",
    resync: "Resincronizar",
    close: "Cerrar",
    sync: "Sincronizar",
    manage_credentials: "Credenciales",
    bulk_assign: "Asignación masiva",
}

export function resourceLabel(resource: string): string {
    return RESOURCE_LABELS[resource] ?? resource
}

export function actionLabel(action: string): string {
    return ACTION_LABELS[action] ?? action
}

export type PermissionGroup = { resource: string; label: string; permissions: Permission[] }

/** Agrupa el catálogo por recurso, ordenado por etiqueta. */
export function groupPermissions(permissions: Permission[]): PermissionGroup[] {
    const byResource = new Map<string, Permission[]>()
    for (const permission of permissions) {
        const list = byResource.get(permission.resource) ?? []
        list.push(permission)
        byResource.set(permission.resource, list)
    }
    return [...byResource.entries()]
        .map(([resource, list]) => ({ resource, label: resourceLabel(resource), permissions: list }))
        .sort((a, b) => a.label.localeCompare(b.label, "es"))
}

/** Catálogo completo de permisos asignables (GET /api/permissions). */
export function usePermissionsCatalog(enabled = true) {
    return useQuery({
        queryKey: ["permissions-catalog"],
        enabled,
        staleTime: 5 * 60_000,
        queryFn: async () => {
            const response = await apiClient.get("/api/permissions")
            return permissionsResponseSchema.parse(response.data).permissions
        },
    })
}

/** Lista de roles con conteo de permisos y usuarios (GET /api/roles). */
export function useRoles(enabled = true) {
    return useQuery({
        queryKey: ["roles"],
        enabled,
        queryFn: async () => {
            const response = await apiClient.get("/api/roles")
            return rolesResponseSchema.parse(response.data).roles
        },
    })
}

/** Un rol con sus ids de permiso (GET /api/roles/{id}). */
export function useRole(id: number | null) {
    return useQuery({
        queryKey: ["roles", id],
        enabled: id !== null,
        queryFn: async () => {
            const response = await apiClient.get(`/api/roles/${id}`)
            return roleSchema.parse(response.data)
        },
    })
}

export type RoleInput = { name: string; description: string; permissionIds: number[] }

export function useSaveRole() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: RoleInput & { id?: number }) => {
            const { id, ...body } = input
            const response = id
                ? await apiClient.put(`/api/roles/${id}`, body)
                : await apiClient.post("/api/roles", body)
            return roleSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["roles"] })
            queryClient.invalidateQueries({ queryKey: ["users"] })
        },
    })
}

export function useDeleteRole() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (id: number) => {
            await apiClient.delete(`/api/roles/${id}`)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["roles"] })
        },
    })
}

/** Usuario con sus excepciones de permisos (GET /api/users/{id}). */
export function useUserDetail(id: number | null) {
    return useQuery({
        queryKey: ["users", "detail", id],
        enabled: id !== null,
        queryFn: async () => {
            const response = await apiClient.get(`/api/users/${id}`)
            return userDetailSchema.parse(response.data)
        },
    })
}

export type UserAccessInput = {
    isAdmin: boolean
    roleId: number | null
    isActive: boolean
    overrides: PermissionOverride[]
}

export function useUpdateUserAccess() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async ({ id, ...body }: UserAccessInput & { id: number }) => {
            const response = await apiClient.put(`/api/users/${id}`, body)
            return userDetailSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users"] })
            queryClient.invalidateQueries({ queryKey: ["roles"] })
        },
    })
}

import { apiClient } from "@/lib/api"
import type { PermissionOverride } from "@/lib/schemas/rbac"
import { paginatedUsersResponseSchema, userSchema } from "@/lib/schemas/users"
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

// Hooks de la página de Usuarios (tabla paginada + alta con contraseña).

/** Lista paginada de usuarios (GET /api/users). */
export function useUsers(offset: number, pageSize: number) {
    return useQuery({
        queryKey: ["users", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/users", { params: { offset, pageSize } })
            return paginatedUsersResponseSchema.parse(response.data)
        },
    })
}

/**
 * Alta de usuario (POST /api/users): cuenta administradora (acceso total) o
 * con rol + excepciones de permisos (ADR 0009).
 */
export function useCreateUser() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: {
            username: string
            password: string
            isAdmin: boolean
            roleId: number | null
            overrides: PermissionOverride[]
        }) => {
            const response = await apiClient.post("/api/users", input)
            return userSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users"] })
        },
    })
}

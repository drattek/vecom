import { apiClient } from "@/lib/api"
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
 * Alta de usuario (POST /api/users). El rol no se elige desde acá — el
 * backend lo fija en "admin" hasta que exista un sistema de roles/permisos.
 */
export function useCreateUser() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { username: string; password: string }) => {
            const response = await apiClient.post("/api/users", input)
            return userSchema.parse(response.data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users"] })
        },
    })
}

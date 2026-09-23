import { PageHeader } from "@/components/PageHeader.tsx"
import { TablePagination } from "@/components/TablePagination"
import { CreateUserDialog } from "@/components/users/CreateUserDialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useUsers } from "@/lib/users"
import { useUsersPaginationStore } from "@/stores/usersPaginationStore"

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

export function UsersPage() {
    const offset = useUsersPaginationStore((state) => state.offset)
    const pageSize = useUsersPaginationStore((state) => state.pageSize)
    const setOffset = useUsersPaginationStore((state) => state.setOffset)
    const setPageSize = useUsersPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useUsers(offset, pageSize)

    const users = data?.users ?? []
    const total = data?.total ?? 0

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Usuarios</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <div className="flex shrink-0 items-start justify-between gap-3">
                    <div>
                        <h2 className="text-xl font-semibold text-foreground">Usuarios</h2>
                        <p className="mt-2 text-sm text-muted-foreground">Cuentas con acceso al panel de administración.</p>
                    </div>
                    <CreateUserDialog />
                </div>

                {isError ? (
                    <p className="mt-2 text-sm text-destructive">
                        Error al obtener los usuarios: {error instanceof Error ? error.message : "Error desconocido"}
                    </p>
                ) : null}

                <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                    <TableHeader>
                        <TableRow>
                            <TableHead>Usuario</TableHead>
                            <TableHead>Rol</TableHead>
                            <TableHead>Estado</TableHead>
                            <TableHead>Fecha de creación</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading && users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="text-center text-muted-foreground">
                                    Cargando usuarios...
                                </TableCell>
                            </TableRow>
                        ) : null}

                        {!isLoading && users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="text-center text-muted-foreground">
                                    No se encontraron usuarios.
                                </TableCell>
                            </TableRow>
                        ) : null}

                        {users.map((user) => (
                            <TableRow key={user.id}>
                                <TableCell>{user.username}</TableCell>
                                <TableCell className="text-muted-foreground">{user.role}</TableCell>
                                <TableCell>
                                    {user.isActive ? (
                                        <span className="text-emerald-600">Activo</span>
                                    ) : (
                                        <span className="text-muted-foreground">Inactivo</span>
                                    )}
                                </TableCell>
                                <TableCell>{formatDate(user.createdAt)}</TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>

                <TablePagination
                    offset={offset}
                    pageSize={pageSize}
                    total={total}
                    onOffsetChange={setOffset}
                    onPageSizeChange={setPageSize}
                />
            </section>
        </main>
    )
}

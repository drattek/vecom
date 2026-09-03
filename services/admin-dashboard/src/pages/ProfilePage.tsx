import { PageHeader } from '@/components/PageHeader.tsx'
import { useAuth } from '@/auth/AuthContext.tsx'

export function ProfilePage() {
  const { session } = useAuth()
  const user = session?.user

  return (
    <main className="p-2">
      <PageHeader>
        <p>Mi perfil</p>
      </PageHeader>

      <section className="m-4 max-w-xl rounded-xl border bg-card p-6">
        <h2 className="text-xl font-semibold text-foreground">Datos de la sesion</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Informacion del usuario con el que iniciaste sesion.
        </p>

        <dl className="mt-6 grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 text-sm">
          <dt className="font-medium text-muted-foreground">Usuario</dt>
          <dd className="text-foreground">{user?.username ?? '-'}</dd>

          <dt className="font-medium text-muted-foreground">Rol</dt>
          <dd className="text-foreground">{user?.role ?? '-'}</dd>

          <dt className="font-medium text-muted-foreground">ID</dt>
          <dd className="text-foreground">{user?.id ?? '-'}</dd>

          <dt className="font-medium text-muted-foreground">Estado</dt>
          <dd className="text-foreground">{user?.isActive ? 'Activo' : 'Inactivo'}</dd>
        </dl>
      </section>
    </main>
  )
}

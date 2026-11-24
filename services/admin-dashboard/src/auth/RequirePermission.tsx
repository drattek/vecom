import type { PropsWithChildren } from 'react'
import { ShieldAlertIcon } from 'lucide-react'
import { useAuth } from './AuthContext.tsx'

interface RequirePermissionProps {
  /** Basta con tener uno de estos permisos "recurso.accion" (admin siempre pasa). */
  any?: string[]
  /** Solo administradores (rutas sin permiso asignable). */
  adminOnly?: boolean
}

/** Guarda de ruta/sección por permiso. Muestra un aviso si no hay acceso. */
export function RequirePermission({ any = [], adminOnly = false, children }: PropsWithChildren<RequirePermissionProps>) {
  const { can, isAdmin } = useAuth()
  const allowed = isAdmin || (!adminOnly && can(...any))

  if (!allowed) {
    return (
      <main className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center">
        <ShieldAlertIcon className="size-8 text-muted-foreground" />
        <h2 className="text-lg font-semibold text-foreground">Sin acceso</h2>
        <p className="max-w-sm text-sm text-muted-foreground">
          Tu rol no tiene permiso para ver esta sección. Pide a un administrador que te lo asigne.
        </p>
      </main>
    )
  }

  return <>{children}</>
}

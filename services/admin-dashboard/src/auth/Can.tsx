import type { PropsWithChildren } from 'react'
import { useAuth } from './AuthContext.tsx'

/** Renderiza a sus hijos solo si el usuario es admin o tiene alguno de los permisos. */
export function Can({ any, children }: PropsWithChildren<{ any: string[] }>) {
  const { can } = useAuth()
  return can(...any) ? <>{children}</> : null
}

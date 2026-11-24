export interface AuthUser {
  id: number
  username: string
  role: string
  roleId?: number | null
  roleName?: string
  isActive: boolean
  /** Permisos efectivos "recurso.accion". Vacío para admin (acceso total). */
  permissions?: string[]
}

export interface LoginResponse {
  accessToken: string
  tokenType: string
  expiresAt: string
  user: AuthUser
}

export interface AuthSession {
  accessToken: string
  tokenType: string
  expiresAt: string
  user: AuthUser
}

export interface AuthUser {
  id: number
  username: string
  role: string
  isActive: boolean
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

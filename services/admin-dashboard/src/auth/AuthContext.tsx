import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react'
import { apiClient } from '../lib/api.ts'
import type { AuthSession, AuthUser, LoginResponse } from '../types/auth.ts'
import {
  clearStoredSession,
  getStoredSession,
  setStoredSession,
} from './sessionStorage.ts'

interface AuthContextValue {
  session: AuthSession | null
  isAuthenticated: boolean
  login: (payload: LoginResponse) => void
  logout: () => void
  /** Admin: acceso total, sin consultar permisos (igual que el backend). */
  isAdmin: boolean
  /** true si el usuario es admin o tiene AL MENOS uno de los permisos dados. */
  can: (...permissions: string[]) => boolean
  /** Vuelve a pedir el usuario y sus permisos (GET /api/me). */
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

function isSessionStillValid(session: AuthSession | null): session is AuthSession {
  if (!session || !session.accessToken.trim()) {
    return false
  }

  const expiresAtMillis = Date.parse(session.expiresAt)
  return Number.isFinite(expiresAtMillis) && expiresAtMillis > Date.now()
}

export function AuthProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<AuthSession | null>(() => {
    const stored = getStoredSession()
    return isSessionStillValid(stored) ? stored : null
  })

  const logout = useCallback(() => {
    setSession(null)
    clearStoredSession()
  }, [])

  const login = useCallback((payload: LoginResponse) => {
    const nextSession: AuthSession = {
      accessToken: payload.accessToken,
      tokenType: payload.tokenType,
      expiresAt: payload.expiresAt,
      user: payload.user,
    }

    if (!isSessionStillValid(nextSession)) {
      logout()
      return
    }

    setSession(nextSession)
    setStoredSession(nextSession)
  }, [logout])

  const applyUser = useCallback((user: AuthUser) => {
    setSession((current) => {
      if (!current) {
        return current
      }
      const next: AuthSession = { ...current, user }
      setStoredSession(next)
      return next
    })
  }, [])

  const refreshUser = useCallback(async () => {
    const response = await apiClient.get<AuthUser>('/api/me')
    applyUser(response.data)
  }, [applyUser])

  // Al cargar la app los permisos guardados pueden estar desactualizados
  // (cambio de rol, sesión vieja sin permisos): se refrescan desde el backend.
  const hasSession = session !== null
  useEffect(() => {
    if (!hasSession) {
      return
    }
    let cancelled = false
    apiClient
      .get<AuthUser>('/api/me')
      .then((response) => {
        if (!cancelled) {
          applyUser(response.data)
        }
      })
      .catch(() => {
        // Un 401 ya cierra la sesión vía registerUnauthorizedHandler; otros
        // errores dejan los permisos guardados.
      })
    return () => {
      cancelled = true
    }
  }, [hasSession, applyUser])

  useEffect(() => {
    if (!isSessionStillValid(session)) {
      if (session) {
        logout()
      }
      return
    }

    const expiresAtMillis = Date.parse(session.expiresAt)
    const timer = window.setTimeout(() => {
      logout()
    }, Math.max(expiresAtMillis - Date.now(), 0))

    return () => {
      window.clearTimeout(timer)
    }
  }, [session, logout])

  const value = useMemo<AuthContextValue>(() => {
    const user = session?.user
    const isAdmin = user?.role === 'admin'
    const granted = new Set(user?.permissions ?? [])

    return {
      session,
      isAuthenticated: isSessionStillValid(session),
      login,
      logout,
      isAdmin,
      can: (...permissions: string[]) =>
        isAdmin || permissions.some((permission) => granted.has(permission)),
      refreshUser,
    }
  }, [session, login, logout, refreshUser])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)

  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }

  return context
}

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react'
import type { AuthSession, LoginResponse } from '../types/auth.ts'
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
    return {
      session,
      isAuthenticated: isSessionStillValid(session),
      login,
      logout,
    }
  }, [session, login, logout])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)

  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }

  return context
}

import type { AuthSession } from '../types/auth.ts'

const SESSION_KEY = 'admin-dashboard-session'

function parseSession(rawValue: string | null): AuthSession | null {
  if (!rawValue) {
    return null
  }

  try {
    const parsed = JSON.parse(rawValue) as Partial<AuthSession>

    if (
      typeof parsed.accessToken !== 'string' ||
      typeof parsed.tokenType !== 'string' ||
      typeof parsed.expiresAt !== 'string' ||
      !parsed.user
    ) {
      return null
    }

    return {
      accessToken: parsed.accessToken,
      tokenType: parsed.tokenType,
      expiresAt: parsed.expiresAt,
      user: parsed.user,
    }
  } catch {
    return null
  }
}

export function getStoredSession(): AuthSession | null {
  return parseSession(localStorage.getItem(SESSION_KEY))
}

export function setStoredSession(session: AuthSession): void {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session))
}

export function clearStoredSession(): void {
  localStorage.removeItem(SESSION_KEY)
}

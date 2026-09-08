import axios from 'axios'
import { getStoredSession } from '../auth/sessionStorage.ts'

let onUnauthorized: () => void = () => undefined

export function registerUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler
}

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/',
  timeout: 12_000,
})

apiClient.interceptors.request.use((config) => {
  const session = getStoredSession()

  if (session?.accessToken) {
    config.headers.set(
      'Authorization',
      `${session.tokenType} ${session.accessToken}`,
    )
  }

  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      onUnauthorized()
    }

    return Promise.reject(error)
  },
)

/**
 * Extrae el mensaje de error que envía el core (`{ "error": "..." }`) de un
 * fallo de axios, cayendo a `fallback` cuando no hay respuesta útil (error de
 * red, timeout, forma inesperada).
 */
export function getServerErrorMessage(error: unknown, fallback: string): string {
  if (!axios.isAxiosError(error)) {
    return fallback
  }

  const responseMessage = error.response?.data?.error
  if (typeof responseMessage === 'string' && responseMessage.trim()) {
    return responseMessage
  }

  return fallback
}

/**
 * Revokes the current access token server-side (ecom_api_token.revoked_at).
 * Safe to call even if the token is already invalid — the caller should clear
 * the local session regardless of the outcome.
 */
export async function logoutRequest(): Promise<void> {
  await apiClient.post('/api/logout')
}

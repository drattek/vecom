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

  // Sin respuesta del servidor (timeout o corte) o un 502/503/504 del proxy: la
  // operación pudo seguir en el servidor, así que el mensaje genérico confunde.
  const status = error.response?.status
  const noResponse = !error.response && (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT' || error.code === 'ERR_NETWORK')
  if (noResponse || status === 502 || status === 503 || status === 504) {
    return `${fallback} El servidor tardó demasiado en responder o se cortó la conexión; la operación puede haber continuado en segundo plano, revisa el historial.`
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

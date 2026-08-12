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

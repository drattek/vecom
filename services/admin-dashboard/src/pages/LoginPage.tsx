import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext.tsx'
import { apiClient, getServerErrorMessage } from '../lib/api.ts'
import type { LoginResponse } from '../types/auth.ts'

interface LoginRequest {
  username: string
  password: string
}

const LOGIN_ERROR_FALLBACK = 'No fue posible iniciar sesion. Intenta nuevamente.'

export function LoginPage() {
  const { isAuthenticated, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage('')

    const usernameTrimmed = username.trim()
    const passwordTrimmed = password.trim()

    if (!usernameTrimmed || !passwordTrimmed) {
      setErrorMessage('Usuario y contrasena son obligatorios.')
      return
    }

    setIsLoading(true)

    const payload: LoginRequest = {
      username: usernameTrimmed,
      password: passwordTrimmed,
    }

    try {
      const response = await apiClient.post<LoginResponse>('/api/login', payload)
      login(response.data)

      const locationState = location.state as { from?: string } | null
      const nextRoute =
        typeof locationState?.from === 'string' ? locationState.from : '/dashboard'

      navigate(nextRoute, { replace: true })
    } catch (error) {
      setErrorMessage(getServerErrorMessage(error, LOGIN_ERROR_FALLBACK))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <main className="page-shell">
      <section className="auth-card" aria-labelledby="login-title">
        <p className="eyebrow">Admins Dashboard</p>
        <h1 id="login-title" className="title">
          Iniciar sesion
        </h1>
        <p className="subtitle">Accede con tu credenciales para administrar productos.</p>

        <form onSubmit={handleSubmit} noValidate>
          <div className="field">
            <label htmlFor="username">Usuario</label>
            <input
              id="username"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              disabled={isLoading}
            />
          </div>

          <div className="field">
            <label htmlFor="password">Contrasena</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={isLoading}
            />
          </div>

          {errorMessage && (
            <p className="error-text" role="alert">
              {errorMessage}
            </p>
          )}

          <button className="submit" type="submit" disabled={isLoading}>
            {isLoading ? 'Ingresando...' : 'Entrar'}
          </button>
        </form>
      </section>
    </main>
  )
}

import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext.tsx'
import { apiClient, getServerErrorMessage } from '../lib/api.ts'
import { Button } from '../components/ui/button.tsx'
import { Input } from '../components/ui/input.tsx'
import { Label } from '../components/ui/label.tsx'
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
  // Sin funcionalidad por el momento: solo se conserva el control visual.
  const [rememberMe, setRememberMe] = useState(false)
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
    <main className="grid min-h-svh w-full bg-[var(--bg)] p-4 lg:p-6">
      <div className="mx-auto grid w-full max-w-6xl overflow-hidden rounded-xl border border-[var(--line)] bg-[var(--surface)] shadow-[var(--shadow)] lg:grid-cols-2">
        {/* Panel del formulario */}
        <div className="flex flex-col px-6 py-10 sm:px-12 lg:px-16 lg:py-14">
          {/* Espacio reservado para el logo (pendiente) */}
          <div className="mb-12 flex h-10 items-center">
            <div
              className="flex h-10 w-36 items-center justify-center rounded-md border border-dashed border-[var(--line)] text-xs font-medium text-muted-foreground"
              aria-label="Espacio reservado para el logo"
            >
              Logo
            </div>
          </div>

          <div className="flex flex-1 flex-col justify-center">
            <div className="mx-auto w-full max-w-sm">
              <h1
                className="text-2xl font-bold tracking-tight text-[var(--text)]"
                style={{ fontFamily: 'var(--font-heading)' }}
              >
                Inicia sesion en tu cuenta
              </h1>
              <p className="mt-1.5 text-sm text-muted-foreground">
                Ingresa tus datos para continuar.
              </p>

              <form onSubmit={handleSubmit} noValidate className="mt-8 space-y-5">
                <div className="space-y-2">
                  <Label htmlFor="username">Usuario</Label>
                  <Input
                    id="username"
                    autoComplete="username"
                    placeholder="Ingresa tu usuario"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    disabled={isLoading}
                    className="h-10"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="password">Contrasena</Label>
                  <Input
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    placeholder="Ingresa tu contrasena"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    disabled={isLoading}
                    className="h-10"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <label className="flex items-center gap-2 text-sm text-[var(--text)] select-none">
                    <input
                      type="checkbox"
                      checked={rememberMe}
                      onChange={(event) => setRememberMe(event.target.checked)}
                      className="size-4 rounded border-[var(--line)] accent-[var(--brand)]"
                    />
                    Recordarme
                  </label>

                  <button
                    type="button"
                    className="text-sm font-medium text-[var(--brand)] hover:underline"
                    onClick={(event) => event.preventDefault()}
                  >
                    Olvide mi contrasena
                  </button>
                </div>

                {errorMessage && (
                  <p className="text-sm text-[var(--danger)]" role="alert">
                    {errorMessage}
                  </p>
                )}

                <Button
                  type="submit"
                  size="lg"
                  disabled={isLoading}
                  className="h-10 w-full"
                >
                  {isLoading ? 'Ingresando...' : 'Iniciar sesion'}
                </Button>
              </form>
            </div>
          </div>
        </div>

        {/* Espacio reservado para la imagen lateral (pendiente) */}
        <div
          className="hidden bg-muted lg:block"
          aria-hidden="true"
        />
      </div>
    </main>
  )
}

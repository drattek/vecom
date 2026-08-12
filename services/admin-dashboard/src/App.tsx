import { useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SettingsPage } from './pages/SettingsPage.tsx'
import { AuthProvider, useAuth } from './auth/AuthContext.tsx'
import { ProtectedRoute } from './auth/ProtectedRoute.tsx'
import { registerUnauthorizedHandler } from './lib/api.ts'
import { DashboardPage } from './pages/DashboardPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { ProductsPage } from './pages/ProductsPage.tsx'
import { UsersPage } from './pages/UsersPage.tsx'
import { ProtectedLayout } from './layouts/ProtectedLayout.tsx'
import { TooltipProvider } from './components/ui/tooltip.tsx'
import { SettingsSectionPage } from './pages/settings/SettingsSectionPage.tsx'
import { ChannelsPage } from './pages/settings/ChannelsPage.tsx'
import { StorageDiskPage } from './pages/settings/StorageDiskPage.tsx'
import { FilesPage } from './pages/settings/FilesPage.tsx'

const queryClient = new QueryClient()

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <ProtectedRoute>
            <ProtectedLayout />
          </ProtectedRoute>
        }
      >
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/settings" element={<SettingsPage />}>
          <Route
            index
            element={<Navigate to="marketplace-channels" replace />}
          />
          <Route
            path="storage-disks"
            element={<StorageDiskPage />}
          />
          <Route
            path="storage-files"
            element={<FilesPage />}
          />
          <Route
            path="company-branches"
            element={
              <SettingsSectionPage
                title="Sucursales"
                description="Gestiona la estructura de sucursales y su disponibilidad operativa."
              />
            }
          />
          <Route
            path="company-warehouses"
            element={
              <SettingsSectionPage
                title="Almacenes"
                description="Define almacenes, capacidad y asignaciones para inventario."
              />
            }
          />
          <Route
            path="company-currencies"
            element={
              <SettingsSectionPage
                title="Divisas"
                description="Mantiene las divisas disponibles para precios, compras y reportes."
              />
            }
          />
          <Route
            path="company-exchange-rates"
            element={
              <SettingsSectionPage
                title="Tipo de cambio"
                description="Actualiza y consulta tipos de cambio para conversiones monetarias."
              />
            }
          />
          <Route
            path="marketplace-channels"
            element={<ChannelsPage />}
          />
          <Route
            path="marketplace-connections"
            element={
              <SettingsSectionPage
                title="Conexiones"
                description="Administra credenciales y estados de conexión con marketplaces."
              />
            }
          />
        </Route>
      </Route>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route
        path="*"
        element={
          <ProtectedRoute>
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}

function AuthSideEffects() {
  const { logout } = useAuth()

  useEffect(() => {
    registerUnauthorizedHandler(logout)
  }, [logout])

  return null
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <TooltipProvider>
            <AuthSideEffects />
            <AppRoutes />
          </TooltipProvider>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App

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
import { ProductsChannelPage } from './pages/ProductsChannelPage.tsx'
import { ProductsChannelDashboard } from './pages/ProductsChannelDashboard.tsx'
import { ProductsChannelConnectionPage } from './pages/ProductsChannelConnectionPage.tsx'
import { ProductsPendingPage } from './pages/ProductsPendingPage.tsx'
import { ProfilePage } from './pages/ProfilePage.tsx'
import { UsersPage } from './pages/UsersPage.tsx'
import { ProtectedLayout } from './layouts/ProtectedLayout.tsx'
import { TooltipProvider } from './components/ui/tooltip.tsx'
import { ChannelsPage } from './pages/settings/ChannelsPage.tsx'
import { StorageDiskPage } from './pages/settings/StorageDiskPage.tsx'
import { FilesPage } from './pages/settings/FilesPage.tsx'
import { BranchesPage } from './pages/settings/BranchesPage.tsx'
import { WarehousesPage } from './pages/settings/WarehousesPage.tsx'
import { ConnectionsPage } from './pages/settings/ConnectionsPage.tsx'
import { BrandsPage } from './pages/settings/BrandsPage.tsx'
import { CurrenciesPage } from './pages/settings/CurrenciesPage.tsx'
import { ExchangeRatesPage } from './pages/settings/ExchangeRatesPage.tsx'
import { CategoriesPage } from './pages/settings/CategoriesPage.tsx'

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
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/products/pending" element={<ProductsPendingPage />} />
        <Route path="/products/channels/:channelId" element={<ProductsChannelPage />}>
          <Route index element={<ProductsChannelDashboard />} />
          <Route path="connections/:connectionId" element={<ProductsChannelConnectionPage />} />
        </Route>
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
            element={<BranchesPage />}
          />
          <Route
            path="company-warehouses"
            element={<WarehousesPage />}
          />
          <Route
            path="company-currencies"
            element={<CurrenciesPage />}
          />
          <Route
            path="company-exchange-rates"
            element={<ExchangeRatesPage />}
          />
          <Route
            path="marketplace-channels"
            element={<ChannelsPage />}
          />
          <Route
            path="marketplace-connections"
            element={<ConnectionsPage />}
          />
          <Route
            path="marketplace-brands"
            element={<BrandsPage />}
          />
          <Route
            path="marketplace-categories"
            element={<CategoriesPage />}
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

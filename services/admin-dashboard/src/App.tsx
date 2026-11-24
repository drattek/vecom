import { useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SettingsIndexRedirect, SettingsPage } from './pages/SettingsPage.tsx'
import { AuthProvider, useAuth } from './auth/AuthContext.tsx'
import { ProtectedRoute } from './auth/ProtectedRoute.tsx'
import { RequirePermission } from './auth/RequirePermission.tsx'
import { HomeGate } from './auth/HomeGate.tsx'
import { registerUnauthorizedHandler } from './lib/api.ts'
import { DashboardPage } from './pages/DashboardPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { ProductsPage } from './pages/ProductsPage.tsx'
import { ProductsChannelPage } from './pages/ProductsChannelPage.tsx'
import { ProductsChannelDashboard } from './pages/ProductsChannelDashboard.tsx'
import { ProductsChannelConnectionPage } from './pages/ProductsChannelConnectionPage.tsx'
import { ProductsPendingPage } from './pages/ProductsPendingPage.tsx'
import { ProductsImagesPage } from './pages/ProductsImagesPage.tsx'
import { ProductsImportPage } from './pages/ProductsImportPage.tsx'
import { ProductsBulkUpdatePage } from './pages/ProductsBulkUpdatePage.tsx'
import { ProductDetailPage } from './pages/ProductDetailPage.tsx'
import { ProductGeneralSection } from './pages/product-detail/ProductGeneralSection.tsx'
import { ProductMediaSection } from './pages/product-detail/ProductMediaSection.tsx'
import { ProductPricingSection } from './pages/product-detail/ProductPricingSection.tsx'
import { ProductInventorySection } from './pages/product-detail/ProductInventorySection.tsx'
import { ProductPartNumbersSection } from './pages/product-detail/ProductPartNumbersSection.tsx'
import { ProductAttributesSection } from './pages/product-detail/ProductAttributesSection.tsx'
import { ProductCompatibilitiesSection } from './pages/product-detail/ProductCompatibilitiesSection.tsx'
import { ProductSyncSection } from './pages/product-detail/ProductSyncSection.tsx'
import { ProfilePage } from './pages/ProfilePage.tsx'
import { UsersPage } from './pages/UsersPage.tsx'
import { MeliListingsPage } from './pages/meli/MeliListingsPage.tsx'
import { PublicationsReportPage } from './pages/reports/PublicationsReportPage.tsx'
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
import { PriceListsPage } from './pages/settings/PriceListsPage.tsx'
import { PricingFormulasPage } from './pages/settings/PricingFormulasPage.tsx'
import { CategoriesPage } from './pages/settings/CategoriesPage.tsx'
import { CategoryDetailPage } from './pages/settings/CategoryDetailPage.tsx'
import { FitmentsVehiclesPage } from './pages/settings/FitmentsVehiclesPage.tsx'
import { FitmentsEquipmentPage } from './pages/settings/FitmentsEquipmentPage.tsx'
import { EquipmentTypesPage } from './pages/settings/EquipmentTypesPage.tsx'
import { RolesPage } from './pages/settings/RolesPage.tsx'

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
        <Route path="/dashboard" element={<HomeGate><DashboardPage /></HomeGate>} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/users" element={<RequirePermission any={['users.view']}><UsersPage /></RequirePermission>} />
        <Route path="/meli" element={<Navigate to="/meli/listings" replace />} />
        <Route path="/meli/listings" element={<RequirePermission any={['meli_listings.view']}><MeliListingsPage /></RequirePermission>} />
        <Route path="/reports" element={<Navigate to="/reports/publications" replace />} />
        <Route path="/reports/publications" element={<RequirePermission any={['reports.view']}><PublicationsReportPage /></RequirePermission>} />
        <Route path="/products" element={<RequirePermission any={['products.view']}><ProductsPage /></RequirePermission>} />
        <Route path="/products/pending" element={<RequirePermission any={['products_pending.view']}><ProductsPendingPage /></RequirePermission>} />
        <Route path="/products/images" element={<RequirePermission any={['products_images.view']}><ProductsImagesPage /></RequirePermission>} />
        <Route path="/products/import" element={<RequirePermission any={['products_import.view']}><ProductsImportPage /></RequirePermission>} />
        <Route path="/products/bulk-update" element={<RequirePermission any={['products_bulk_update.view']}><ProductsBulkUpdatePage /></RequirePermission>} />
        <Route path="/products/channels/:channelId" element={<RequirePermission any={['products_channels.view']}><ProductsChannelPage /></RequirePermission>}>
          <Route index element={<ProductsChannelDashboard />} />
          <Route path="connections/:connectionId" element={<ProductsChannelConnectionPage />} />
        </Route>
        {/*
          Detalle de producto. Va después de /products/pending y
          /products/channels/... a propósito: react-router prioriza los
          segmentos estáticos sobre :productId, así que esas rutas siguen
          ganando y este patrón solo captura ids reales.
        */}
        <Route path="/products/:productId" element={<RequirePermission any={['products.view']}><ProductDetailPage /></RequirePermission>}>
          <Route index element={<ProductGeneralSection />} />
          <Route path="media" element={<RequirePermission any={['product_media.view']}><ProductMediaSection /></RequirePermission>} />
          <Route path="pricing" element={<RequirePermission any={['product_pricing.view']}><ProductPricingSection /></RequirePermission>} />
          <Route path="inventory" element={<RequirePermission any={['product_inventory.view']}><ProductInventorySection /></RequirePermission>} />
          <Route path="part-numbers" element={<RequirePermission any={['product_part_numbers.view']}><ProductPartNumbersSection /></RequirePermission>} />
          <Route path="attributes" element={<RequirePermission any={['product_attributes.view']}><ProductAttributesSection /></RequirePermission>} />
          <Route path="compatibilities" element={<RequirePermission any={['product_compatibilities.view']}><ProductCompatibilitiesSection /></RequirePermission>} />
          <Route path="sync" element={<RequirePermission any={['product_sync.view']}><ProductSyncSection /></RequirePermission>} />
        </Route>
        <Route path="/settings" element={<SettingsPage />}>
          <Route index element={<SettingsIndexRedirect />} />
          <Route
            path="storage-disks"
            element={<RequirePermission any={['storage_disks.view']}><StorageDiskPage /></RequirePermission>}
          />
          <Route
            path="storage-files"
            element={<RequirePermission any={['storage_files.view']}><FilesPage /></RequirePermission>}
          />
          <Route
            path="company-branches"
            element={<RequirePermission any={['branches.view']}><BranchesPage /></RequirePermission>}
          />
          <Route
            path="company-warehouses"
            element={<RequirePermission any={['warehouses.view']}><WarehousesPage /></RequirePermission>}
          />
          <Route
            path="company-currencies"
            element={<RequirePermission any={['currencies.view']}><CurrenciesPage /></RequirePermission>}
          />
          <Route
            path="company-exchange-rates"
            element={<RequirePermission any={['exchange_rates.view']}><ExchangeRatesPage /></RequirePermission>}
          />
          <Route
            path="pricing-price-lists"
            element={<RequirePermission any={['price_lists.view']}><PriceListsPage /></RequirePermission>}
          />
          <Route
            path="pricing-formulas"
            element={<RequirePermission any={['pricing_formulas.view']}><PricingFormulasPage /></RequirePermission>}
          />
          <Route
            path="marketplace-channels"
            element={<RequirePermission any={['channels.view']}><ChannelsPage /></RequirePermission>}
          />
          <Route
            path="marketplace-connections"
            element={<RequirePermission any={['connections.view']}><ConnectionsPage /></RequirePermission>}
          />
          <Route
            path="marketplace-brands"
            element={<RequirePermission any={['brands.view']}><BrandsPage /></RequirePermission>}
          />
          <Route
            path="marketplace-categories"
            element={<RequirePermission any={['categories.view']}><CategoriesPage /></RequirePermission>}
          />
          <Route
            path="marketplace-categories/:categoryId"
            element={<RequirePermission any={['categories.view']}><CategoryDetailPage /></RequirePermission>}
          />
          <Route
            path="fitments-vehicles"
            element={<RequirePermission any={['vehicle_fitments.view']}><FitmentsVehiclesPage /></RequirePermission>}
          />
          <Route
            path="fitments-equipment"
            element={<RequirePermission any={['equipment_fitments.view']}><FitmentsEquipmentPage /></RequirePermission>}
          />
          <Route
            path="access-roles"
            element={<RequirePermission any={['roles.view']}><RolesPage /></RequirePermission>}
          />
          <Route
            path="fitments-equipment-types"
            element={<RequirePermission any={['equipment_types.view']}><EquipmentTypesPage /></RequirePermission>}
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

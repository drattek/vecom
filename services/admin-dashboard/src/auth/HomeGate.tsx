import type { PropsWithChildren } from 'react'
import { Navigate } from 'react-router-dom'
import { settingsMenuGroups } from '../lib/settings-menu.ts'
import { useAuth } from './AuthContext.tsx'

// Primera pantalla accesible cuando el usuario no puede ver el dashboard.
const fallbackRoutes: Array<{ path: string; permissions: string[] }> = [
  { path: '/products', permissions: ['products.view'] },
  { path: '/products/pending', permissions: ['products_pending.view'] },
  { path: '/products/images', permissions: ['products_images.view'] },
  { path: '/products/import', permissions: ['products_import.view'] },
  { path: '/products/bulk-update', permissions: ['products_bulk_update.view'] },
  { path: '/sales', permissions: ['sales.view'] },
  { path: '/meli/listings', permissions: ['meli_listings.view'] },
  { path: '/reports/publications', permissions: ['reports.view'] },
  { path: '/users', permissions: ['users.view'] },
  ...settingsMenuGroups.flatMap((group) => group.items).map((item) => ({ path: item.to, permissions: [item.permission] })),
]

/** Muestra el dashboard o, sin permiso `dashboard.view`, redirige a la primera sección permitida. */
export function HomeGate({ children }: PropsWithChildren) {
  const { can } = useAuth()

  if (can('dashboard.view')) {
    return children
  }

  const target = fallbackRoutes.find((route) => can(...route.permissions))
  return <Navigate to={target?.path ?? '/profile'} replace />
}

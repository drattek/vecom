import { Link, Outlet, useLocation } from 'react-router-dom'

import { PageHeader } from '@/components/PageHeader.tsx'
import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
  navigationMenuTriggerStyle,
} from '@/components/ui/navigation-menu'

const dashboardViews = [
  { label: 'Publicaciones', to: '/dashboard' },
  { label: 'Salud de la plataforma', to: '/dashboard/health' },
  { label: 'Preparación del catálogo', to: '/dashboard/catalog' },
]

export function DashboardPage() {
  const location = useLocation()

  return (
    <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
      <PageHeader>
        <NavigationMenu>
          <NavigationMenuList>
            {dashboardViews.map((view) => {
              const isActive = location.pathname === view.to

              return (
                <NavigationMenuItem key={view.to}>
                  <NavigationMenuLink
                    className={`${navigationMenuTriggerStyle()} ${isActive ? 'bg-muted' : ''}`}
                    aria-current={isActive ? 'page' : undefined}
                    render={<Link to={view.to} />}
                  >
                    {view.label}
                  </NavigationMenuLink>
                </NavigationMenuItem>
              )
            })}
          </NavigationMenuList>
        </NavigationMenu>
      </PageHeader>

      <section className="min-h-0 flex-1 overflow-auto p-4">
        <Outlet />
      </section>
    </main>
  )
}

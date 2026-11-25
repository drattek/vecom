import { PageHeader } from '@/components/PageHeader.tsx'
import { NavigationMenu, NavigationMenuContent, NavigationMenuItem, NavigationMenuLink, NavigationMenuList, NavigationMenuTrigger } from '@/components/ui/navigation-menu'
import { Link, Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext.tsx'
import { settingsMenuGroups } from '@/lib/settings-menu'

/** Ruta índice de /settings: la primera sección a la que el usuario tiene acceso. */
export function SettingsIndexRedirect() {
    const { can } = useAuth()
    const first = settingsMenuGroups.flatMap((group) => group.items).find((item) => can(item.permission))

    if (!first) {
        return (
            <p className="p-4 text-sm text-muted-foreground">
                Tu rol no tiene acceso a ninguna sección de Configuración.
            </p>
        )
    }
    return <Navigate to={first.to} replace />
}

export function SettingsPage() {
    const location = useLocation()
    const { can } = useAuth()

    const visibleGroups = settingsMenuGroups
        .map((group) => ({ ...group, items: group.items.filter((item) => can(item.permission)) }))
        .filter((group) => group.items.length > 0)

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <NavigationMenu>
                    <NavigationMenuList>
                        {visibleGroups.map((group) => (
                            <NavigationMenuItem key={group.label}>
                                <NavigationMenuTrigger>
                                    <span>{group.label}</span>
                                </NavigationMenuTrigger>
                                <NavigationMenuContent>
                                    <ul className="grid w-55 gap-1">
                                        {group.items.map((item) => {
                                            const isActive = location.pathname === item.to

                                            return (
                                                <li key={item.to}>
                                                    <NavigationMenuLink
                                                        className={isActive ? 'bg-muted/50' : undefined}
                                                        render={<Link to={item.to} />}
                                                    >
                                                        <span>{item.label}</span>
                                                    </NavigationMenuLink>
                                                </li>
                                            )
                                        })}
                                    </ul>
                                </NavigationMenuContent>
                            </NavigationMenuItem>
                        ))}
                    </NavigationMenuList>
                </NavigationMenu>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <Outlet />
            </section>
        </main>
    )
}
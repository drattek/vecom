import { PageHeader } from '@/components/PageHeader.tsx'
import { NavigationMenu, NavigationMenuContent, NavigationMenuItem, NavigationMenuLink, NavigationMenuList, NavigationMenuTrigger } from '@/components/ui/navigation-menu'
import { Link, Outlet, useLocation } from 'react-router-dom'

interface SettingsMenuItem {
    label: string
    to: string
}

interface SettingsMenuGroup {
    label: string
    items: SettingsMenuItem[]
}

const settingsMenuGroups: SettingsMenuGroup[] = [
    {
        label: 'Almacenamiento',
        items: [
            { label: 'Discos', to: '/settings/storage-disks' },
            { label: 'Archivos', to: '/settings/storage-files' },
        ],
    },
    {
        label: 'Empresa',
        items: [
            { label: 'Sucursales', to: '/settings/company-branches' },
            { label: 'Almacenes', to: '/settings/company-warehouses' },
            { label: 'Divisas', to: '/settings/company-currencies' },
            { label: 'Tipo de cambio', to: '/settings/company-exchange-rates' },
        ],
    },
    {
        label: 'Marketplace',
        items: [
            { label: 'Canales', to: '/settings/marketplace-channels' },
            { label: 'Conexiones', to: '/settings/marketplace-connections' },
        ],
    },
]

export function SettingsPage() {
    const location = useLocation()

    return (
        <main className="p-2">
            <PageHeader>
                <NavigationMenu>
                    <NavigationMenuList>
                        {settingsMenuGroups.map((group) => (
                            <NavigationMenuItem key={group.label}>
                                <NavigationMenuTrigger>
                                    <span>{group.label}</span>
                                </NavigationMenuTrigger>
                                <NavigationMenuContent>
                                    <ul className="grid w-[220px] gap-1">
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

            <section className="p-4">
                <Outlet />
            </section>
        </main>
    )
}
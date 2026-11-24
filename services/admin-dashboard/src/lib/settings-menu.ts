// Menú de Configuración (barra superior de /settings). Cada entrada declara el
// permiso "recurso.accion" que la hace visible; sin él no aparece en el menú ni
// se puede abrir la ruta (ver App.tsx). Admin ve todo.

export interface SettingsMenuItem {
    label: string
    to: string
    permission: string
}

export interface SettingsMenuGroup {
    label: string
    items: SettingsMenuItem[]
}

export const settingsMenuGroups: SettingsMenuGroup[] = [
    {
        label: 'Almacenamiento',
        items: [
            { label: 'Discos', to: '/settings/storage-disks', permission: 'storage_disks.view' },
            { label: 'Archivos', to: '/settings/storage-files', permission: 'storage_files.view' },
        ],
    },
    {
        label: 'Empresa',
        items: [
            { label: 'Sucursales', to: '/settings/company-branches', permission: 'branches.view' },
            { label: 'Almacenes', to: '/settings/company-warehouses', permission: 'warehouses.view' },
            { label: 'Divisas', to: '/settings/company-currencies', permission: 'currencies.view' },
            { label: 'Tipo de cambio', to: '/settings/company-exchange-rates', permission: 'exchange_rates.view' },
        ],
    },
    {
        label: 'Precios',
        items: [
            { label: 'Listas de precios', to: '/settings/pricing-price-lists', permission: 'price_lists.view' },
            { label: 'Fórmulas de precios', to: '/settings/pricing-formulas', permission: 'pricing_formulas.view' },
        ],
    },
    {
        label: 'Marketplace',
        items: [
            { label: 'Canales', to: '/settings/marketplace-channels', permission: 'channels.view' },
            { label: 'Conexiones', to: '/settings/marketplace-connections', permission: 'connections.view' },
            { label: 'Marcas', to: '/settings/marketplace-brands', permission: 'brands.view' },
            { label: 'Categorías', to: '/settings/marketplace-categories', permission: 'categories.view' },
        ],
    },
    {
        label: 'Vehículos/Maquinaria',
        items: [
            { label: 'Vehículos', to: '/settings/fitments-vehicles', permission: 'vehicle_fitments.view' },
            { label: 'Maquinaria', to: '/settings/fitments-equipment', permission: 'equipment_fitments.view' },
            { label: 'Tipos de maquinaria', to: '/settings/fitments-equipment-types', permission: 'equipment_types.view' },
        ],
    },
    {
        label: 'Accesos',
        items: [{ label: 'Roles', to: '/settings/access-roles', permission: 'roles.view' }],
    },
]

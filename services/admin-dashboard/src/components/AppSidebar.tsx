import { useState, type ComponentType } from 'react'
import {
  BoxIcon,
  ChevronsUpDownIcon,
  GalleryHorizontal,
  LayoutDashboardIcon,
  LogOutIcon,
  Settings,
  UserRoundIcon,
  UsersIcon,
} from 'lucide-react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from './ui/sidebar.tsx'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from './ui/dropdown-menu.tsx'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './ui/alert-dialog.tsx'
import { Avatar, AvatarFallback } from './ui/avatar.tsx'
import { Button } from './ui/button.tsx'
import { useAuth } from '../auth/AuthContext.tsx'
import { logoutRequest } from '../lib/api.ts'

interface SidebarItem {
  title: string
  path: string
  icon: ComponentType<{ className?: string }>
}

const sidebarItems: SidebarItem[] = [
  {
    title: 'Dashboard',
    path: '/dashboard',
    icon: LayoutDashboardIcon,
  },
  {
    title: 'Users',
    path: '/users',
    icon: UsersIcon,
  },
  {
    title: 'Products',
    path: '/products',
    icon: BoxIcon,
  },
  {
    title: 'Settings',
    path: '/settings',
    icon: Settings,
  },
]

function getInitials(name: string): string {
  const initials = name
    .trim()
    .split(/\s+/)
    .map((part) => part[0] ?? '')
    .slice(0, 2)
    .join('')
    .toUpperCase()

  return initials || '?'
}

export function AppSidebar() {
  const location = useLocation()
  const navigate = useNavigate()
  const { session, logout } = useAuth()

  const username = session?.user.username ?? 'Usuario'
  const role = session?.user.role ?? ''

  const [confirmLogoutOpen, setConfirmLogoutOpen] = useState(false)
  const [loggingOut, setLoggingOut] = useState(false)

  async function handleConfirmLogout() {
    setLoggingOut(true)
    try {
      await logoutRequest()
    } catch {
      // El token se limpia localmente pase lo que pase; si el backend falla
      // igual cerramos la sesión en el cliente.
    } finally {
      setLoggingOut(false)
      setConfirmLogoutOpen(false)
      logout()
      navigate('/login', { replace: true })
    }
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
            <SidebarMenuItem>
                <SidebarMenuButton size="lg">
                    <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                        <GalleryHorizontal className="size-4" />
                    </div>
                    <span>Admin Dashboard</span>
                </SidebarMenuButton>
            </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Navegacion</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {sidebarItems.map((item) => {
                const Icon = item.icon
                const isActive =
                  location.pathname === item.path ||
                  location.pathname.startsWith(`${item.path}/`)

                return (
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton
                      render={<Link to={item.path} />}
                      isActive={isActive}
                      tooltip={item.title}
                    >
                      <Icon />
                      <span>{item.title}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <SidebarMenuButton
                    size="lg"
                    className="data-popup-open:bg-sidebar-accent data-popup-open:text-sidebar-accent-foreground"
                  >
                    <Avatar>
                      <AvatarFallback className="rounded-lg">
                        {getInitials(username)}
                      </AvatarFallback>
                    </Avatar>
                    <div className="grid flex-1 text-left text-sm leading-tight">
                      <span className="truncate font-medium">{username}</span>
                      {role ? (
                        <span className="truncate text-xs text-muted-foreground">
                          {role}
                        </span>
                      ) : null}
                    </div>
                    <ChevronsUpDownIcon className="ml-auto size-4" />
                  </SidebarMenuButton>
                }
              />
              <DropdownMenuContent
                className="min-w-56"
                side="top"
                align="end"
                sideOffset={8}
              >
                <DropdownMenuGroup>
                  <DropdownMenuLabel className="font-normal">
                    <div className="grid text-left text-sm leading-tight">
                      <span className="truncate font-medium text-foreground">
                        {username}
                      </span>
                      {role ? (
                        <span className="truncate text-xs text-muted-foreground">
                          {role}
                        </span>
                      ) : null}
                    </div>
                  </DropdownMenuLabel>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuItem onClick={() => navigate('/profile')}>
                    <UserRoundIcon />
                    Mi perfil
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  variant="destructive"
                  onClick={() => setConfirmLogoutOpen(true)}
                >
                  <LogOutIcon />
                  Cerrar sesion
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <AlertDialog
              open={confirmLogoutOpen}
              onOpenChange={setConfirmLogoutOpen}
            >
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Cerrar sesión</AlertDialogTitle>
                  <AlertDialogDescription>
                    ¿Seguro que quieres cerrar sesión? Tendrás que volver a
                    iniciar sesión para acceder al panel.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel disabled={loggingOut}>
                    Cancelar
                  </AlertDialogCancel>
                  <Button
                    variant="destructive"
                    onClick={() => void handleConfirmLogout()}
                    disabled={loggingOut}
                  >
                    {loggingOut ? 'Cerrando sesión…' : 'Cerrar sesión'}
                  </Button>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}

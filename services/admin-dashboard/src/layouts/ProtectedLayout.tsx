import { Outlet } from 'react-router-dom'
import { AppSidebar } from '../components/AppSidebar.tsx'
import {
  SidebarInset,
  SidebarProvider
} from '../components/ui/sidebar.tsx'

export function ProtectedLayout() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset className="h-svh overflow-hidden">
        <Outlet />
      </SidebarInset>
    </SidebarProvider>
  )
}

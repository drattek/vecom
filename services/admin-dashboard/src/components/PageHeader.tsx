import { SidebarTrigger } from './ui/sidebar.tsx'

export function PageHeader({ children }: { children: React.ReactNode }) {
    return (
        <header className="h-16 flex items-center gap-2 px-4 bg-white/50 backdrop-blur border-b border-line sticky top-0">
            <SidebarTrigger />
            {children}
        </header>
    )
}
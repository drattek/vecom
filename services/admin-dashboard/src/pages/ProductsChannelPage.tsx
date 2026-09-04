import { PageHeader } from "@/components/PageHeader.tsx";
import {
    NavigationMenu,
    NavigationMenuItem,
    NavigationMenuLink,
    NavigationMenuList,
    navigationMenuTriggerStyle,
} from "@/components/ui/navigation-menu";
import { apiClient } from "@/lib/api";
import { channelSchema } from "@/lib/schemas/channels";
import { paginatedChannelConnectionsResponseSchema } from "@/lib/schemas/channel-connections";
import { useQuery } from "@tanstack/react-query";
import { Link, Outlet, useLocation, useParams } from "react-router-dom";

export function ProductsChannelPage() {
    const { channelId } = useParams<{ channelId: string }>();
    const location = useLocation();

    const { data: channel, isLoading, isError } = useQuery({
        queryKey: ["channel", channelId],
        enabled: Boolean(channelId),
        queryFn: async () => {
            const response = await apiClient.get(`/api/channels/${channelId}`);
            return channelSchema.parse(response.data);
        },
    });

    const { data: connectionsData } = useQuery({
        queryKey: ["channel-connections", "by-channel", channelId],
        enabled: Boolean(channelId),
        queryFn: async () => {
            const response = await apiClient.get("/api/channel-connections", {
                params: { offset: 0, pageSize: 100 },
            });
            return paginatedChannelConnectionsResponseSchema.parse(response.data);
        },
    });

    const connections = (connectionsData?.channelConnections ?? []).filter(
        (connection) => connection.channelId === Number(channelId) && connection.status === "active",
    );

    const title = isLoading ? "Cargando..." : isError ? "Canal no encontrado" : (channel?.name ?? "Canal");

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <PageHeader>
                <h1 className="whitespace-nowrap text-lg font-semibold text-foreground">{title}</h1>

                {connections.length > 0 ? (
                    <NavigationMenu className="ml-2">
                        <NavigationMenuList>
                            {connections.map((connection) => {
                                const to = `/products/channels/${channelId}/connections/${connection.id}`;
                                const isActive = location.pathname === to;

                                return (
                                    <NavigationMenuItem key={connection.id}>
                                        <NavigationMenuLink
                                            active={isActive}
                                            className={navigationMenuTriggerStyle()}
                                            render={<Link to={to} />}
                                        >
                                            {connection.name}
                                        </NavigationMenuLink>
                                    </NavigationMenuItem>
                                );
                            })}
                        </NavigationMenuList>
                    </NavigationMenu>
                ) : null}
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <Outlet context={{ connections }} />
            </section>
        </main>
    );
}

import { PageHeader } from "@/components/PageHeader.tsx";
import { ImportImagesExcelDialog } from "@/components/products/ImportImagesExcelDialog";
import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { apiClient } from "@/lib/api";
import { useChannelConnections } from "@/lib/product-detail";
import { paginatedChannelsResponseSchema } from "@/lib/schemas/channels";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

const ALL = "all";

export function ProductsImagesPage() {
    // "No publicado en…": canal y/o conexión. Elegir un canal limpia la conexión.
    const [channelId, setChannelId] = useState<string>(ALL);
    const [connectionId, setConnectionId] = useState<string>(ALL);

    const { data: channelsData } = useQuery({
        queryKey: ["sidebar-product-channels"],
        staleTime: 60_000,
        queryFn: async () => {
            const response = await apiClient.get("/api/channels", { params: { offset: 0, pageSize: 100 } });
            return paginatedChannelsResponseSchema.parse(response.data);
        },
    });
    const { data: allConnections } = useChannelConnections();

    const connections = useMemo(
        () =>
            (allConnections ?? []).filter(
                (connection) =>
                    connection.status === "active" && (channelId === ALL || connection.channelId === Number(channelId)),
            ),
        [allConnections, channelId],
    );

    const channelItems = [{ label: "Todos los canales", value: ALL }, ...(channelsData?.channels ?? []).map((c) => ({ label: c.name, value: String(c.id) }))];
    const connectionItems = [{ label: "Todas las conexiones", value: ALL }, ...connections.map((c) => ({ label: c.name, value: String(c.id) }))];

    const filterParams: Record<string, string | number | boolean> = { hasImage: "true" };
    if (channelId !== ALL) filterParams.unpublishedChannelId = Number(channelId);
    if (connectionId !== ALL) filterParams.unpublishedConnectionId = Number(connectionId);
    const filtered = channelId !== ALL || connectionId !== ALL;

    const toolbar = (
        <div className="flex items-center gap-2">
            <span className="whitespace-nowrap text-sm text-muted-foreground">No publicados en</span>
            <Select
                items={channelItems}
                value={channelId}
                onValueChange={(next) => {
                    setChannelId(next ?? ALL);
                    setConnectionId(ALL);
                }}
            >
                <SelectTrigger className="w-44" aria-label="Filtrar por canal">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent align="start">
                    <SelectGroup>
                        {channelItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
            <Select items={connectionItems} value={connectionId} onValueChange={(next) => setConnectionId(next ?? ALL)}>
                <SelectTrigger className="w-48" aria-label="Filtrar por conexión">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent align="start">
                    <SelectGroup>
                        {connectionItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                        ))}
                    </SelectGroup>
                </SelectContent>
            </Select>
        </div>
    );

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Imágenes</p>
                <div className="ml-auto">
                    <ImportImagesExcelDialog />
                </div>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                {/* key: remonta la tabla al cambiar filtros para volver a la página 1. */}
                <ProductsFilteredTable
                    key={`${channelId}-${connectionId}`}
                    queryKey={["products", "images", channelId, connectionId]}
                    filterParams={filterParams}
                    emptyMessage={filtered ? "Todos los productos con imagen ya están publicados en esta selección." : "No hay productos con imágenes."}
                    toolbar={toolbar}
                />
            </section>
        </main>
    );
}

import { apiClient } from "@/lib/api";
import { channelSyncSummarySchema } from "@/lib/schemas/channel-sync-summary";
import type { ChannelConnection } from "@/lib/schemas/channel-connections";
import { useQuery } from "@tanstack/react-query";
import { useOutletContext, useParams } from "react-router-dom";

function daysAgoLabel(dateStr: string): string {
    const date = new Date(`${dateStr}T00:00:00`);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const diffDays = Math.round((today.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));

    if (diffDays <= 0) return "Hoy";
    if (diffDays === 1) return "Hace 1 día";
    return `Hace ${diffDays} días`;
}

function SummaryCard({ label, value }: { label: string; value: number }) {
    return (
        <div className="rounded-xl border bg-card p-6">
            <p className="text-sm text-muted-foreground">{label}</p>
            <p className="mt-2 text-3xl font-semibold tabular-nums text-foreground">{value}</p>
        </div>
    );
}

export function ProductsChannelDashboard() {
    const { channelId } = useParams<{ channelId: string }>();
    const { connections } = useOutletContext<{ connections: ChannelConnection[] }>();

    const { data: summary, isLoading, isError } = useQuery({
        queryKey: ["channel-sync-summary", channelId],
        enabled: Boolean(channelId) && connections.length > 0,
        queryFn: async () => {
            const response = await apiClient.get(`/api/channels/${channelId}/sync-summary`);
            return channelSyncSummarySchema.parse(response.data);
        },
    });

    if (connections.length === 0) {
        return (
            <section className="rounded-xl border bg-card p-6">
                <h2 className="text-xl font-semibold text-foreground">Sin conexiones</h2>
                <p className="mt-2 text-sm text-muted-foreground">
                    Este canal todavía no tiene conexiones activas.
                </p>
            </section>
        );
    }

    if (isLoading) {
        return <p className="text-sm text-muted-foreground">Cargando resumen...</p>;
    }

    if (isError || !summary) {
        return <p className="text-sm text-destructive">No se pudo cargar el resumen del canal.</p>;
    }

    return (
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <SummaryCard label="Productos sincronizados" value={summary.total} />
                <SummaryCard label="Sin problemas" value={summary.withoutIssues} />
                <SummaryCard label="En revisión" value={summary.underReview} />
            </div>

            <div className="rounded-xl border bg-card p-6">
                <h3 className="text-sm font-medium text-foreground">Marcas sincronizadas</h3>
                {summary.brands.length > 0 ? (
                    <div className="mt-3 flex flex-wrap gap-2">
                        {summary.brands.map((brand) => (
                            <span
                                key={brand.id}
                                className="rounded-md border border-border bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground"
                            >
                                {brand.name}
                            </span>
                        ))}
                    </div>
                ) : (
                    <p className="mt-2 text-sm text-muted-foreground">Todavía no hay marcas sincronizadas.</p>
                )}
            </div>

            <div className="rounded-xl border bg-card p-6">
                <h3 className="text-sm font-medium text-foreground">Productos nuevos sincronizados</h3>
                {summary.recentlyAdded.length > 0 ? (
                    <ul className="mt-3 divide-y divide-border">
                        {summary.recentlyAdded.map((entry) => (
                            <li key={entry.date} className="flex items-center justify-between py-2 text-sm">
                                <span className="text-muted-foreground">{daysAgoLabel(entry.date)}</span>
                                <span className="font-medium tabular-nums text-foreground">
                                    {entry.count} {entry.count === 1 ? "producto" : "productos"}
                                </span>
                            </li>
                        ))}
                    </ul>
                ) : (
                    <p className="mt-2 text-sm text-muted-foreground">No hay altas recientes (últimos 30 días).</p>
                )}
            </div>
        </div>
    );
}

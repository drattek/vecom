import { EnqueueCompatibilitiesButton } from "@/components/products/EnqueueCompatibilitiesButton";
import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";
import { useChannelConnections } from "@/lib/product-detail";
import { useParams } from "react-router-dom";

const ODOO_CHANNEL_CODE = "ODOO";

export function ProductsChannelConnectionPage() {
    const { connectionId } = useParams<{ connectionId: string }>();
    const connections = useChannelConnections();

    const connection = connections.data?.find((item) => String(item.id) === connectionId);
    const isOdoo = connection?.channelCode.toUpperCase() === ODOO_CHANNEL_CODE;

    return (
        <ProductsFilteredTable
            key={connectionId}
            queryKey={["products", "by-connection", connectionId]}
            filterParams={{ connectionId: connectionId ?? "" }}
            emptyMessage="No se han sincronizado productos en esta conexión."
            toolbar={connection && isOdoo ? <EnqueueCompatibilitiesButton connectionId={connection.id} /> : undefined}
        />
    );
}

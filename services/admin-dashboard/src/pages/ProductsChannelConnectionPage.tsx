import { EnqueueCompatibilitiesButton } from "@/components/products/EnqueueCompatibilitiesButton";
import { EnqueueResyncButton } from "@/components/products/EnqueueResyncButton";
import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";
import { useChannelConnections } from "@/lib/product-detail";
import { useParams } from "react-router-dom";

const ODOO_CHANNEL_CODE = "ODOO";
const MERCADOLIBRE_CHANNEL_CODE = "MERCADOLIBRE";

export function ProductsChannelConnectionPage() {
    const { connectionId } = useParams<{ connectionId: string }>();
    const connections = useChannelConnections();

    const connection = connections.data?.find((item) => String(item.id) === connectionId);
    const channelCode = connection?.channelCode.toUpperCase();
    const isOdoo = channelCode === ODOO_CHANNEL_CODE;
    const supportsResync = isOdoo || channelCode === MERCADOLIBRE_CHANNEL_CODE;

    return (
        <ProductsFilteredTable
            key={connectionId}
            queryKey={["products", "by-connection", connectionId]}
            filterParams={{ connectionId: connectionId ?? "" }}
            emptyMessage="No se han sincronizado productos en esta conexión."
            toolbar={
                connection && supportsResync ? (
                    <div className="flex flex-wrap items-center gap-4">
                        <EnqueueResyncButton connectionId={connection.id} />
                        {isOdoo ? <EnqueueCompatibilitiesButton connectionId={connection.id} /> : null}
                    </div>
                ) : undefined
            }
        />
    );
}

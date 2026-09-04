import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";
import { useParams } from "react-router-dom";

export function ProductsChannelConnectionPage() {
    const { connectionId } = useParams<{ connectionId: string }>();

    return (
        <ProductsFilteredTable
            key={connectionId}
            queryKey={["products", "by-connection", connectionId]}
            filterParams={{ connectionId: connectionId ?? "" }}
            emptyMessage="No se han sincronizado productos en esta conexión."
        />
    );
}

import { PageHeader } from "@/components/PageHeader.tsx";
import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";

export function ProductsPendingPage() {
    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Pendientes</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <ProductsFilteredTable
                    queryKey={["products", "pending"]}
                    filterParams={{ pending: "true" }}
                    emptyMessage="No hay productos pendientes de sincronizar."
                />
            </section>
        </main>
    );
}

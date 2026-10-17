import { PageHeader } from "@/components/PageHeader.tsx";
import { ImportImagesExcelDialog } from "@/components/products/ImportImagesExcelDialog";
import { ProductsFilteredTable } from "@/components/products/ProductsFilteredTable";

export function ProductsImagesPage() {
    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Imágenes</p>
                <div className="ml-auto">
                    <ImportImagesExcelDialog />
                </div>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <ProductsFilteredTable
                    queryKey={["products", "images"]}
                    filterParams={{ hasImage: "true" }}
                    emptyMessage="No hay productos con imágenes."
                />
            </section>
        </main>
    );
}

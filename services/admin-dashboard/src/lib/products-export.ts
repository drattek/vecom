import { apiClient } from "@/lib/api"
import { paginatedProductsResponseSchema } from "@/lib/schemas/products"

const EXPORT_PAGE_SIZE = 100

type ExportParams = Record<string, string | number | boolean>

/** Recorre todas las páginas de GET /api/products con los filtros/orden/búsqueda dados. */
async function fetchAllProducts(params: ExportParams) {
    const products = []
    let offset = 0
    let total: number
    do {
        const response = await apiClient.get("/api/products", {
            params: { ...params, offset, pageSize: EXPORT_PAGE_SIZE },
        })
        const page = paginatedProductsResponseSchema.parse(response.data)
        products.push(...page.products)
        total = page.total
        offset += EXPORT_PAGE_SIZE
        if (page.products.length === 0) break
    } while (offset < total)
    return products
}

/**
 * Exporta a .xlsx los productos que devuelve la tabla con sus filtros actuales,
 * sin las columnas de imagen ni de acciones.
 */
export async function exportProductsToExcel(params: ExportParams, fileName: string): Promise<number> {
    // Import dinámico: la librería solo se descarga al exportar.
    const { default: writeExcelFile } = await import("write-excel-file/browser")
    const products = await fetchAllProducts(params)

    const header = ["SKU", "Número de parte", "Nombre", "Marca", "Stock", "Precio", "Moneda"].map((value) => ({
        value,
        fontWeight: "bold" as const,
    }))

    const rows = products.map((product) => {
        const amount = product.price ? Number(product.price.amount) : NaN
        return [
            { type: String, value: product.sku },
            { type: String, value: product.partNumber },
            { type: String, value: product.name },
            { type: String, value: product.brandName ?? "" },
            { type: Number, value: product.totalStock },
            Number.isFinite(amount) ? { type: Number, value: amount, format: "#,##0.00" } : null,
            product.price ? { type: String, value: product.price.currencyCode } : null,
        ]
    })

    await writeExcelFile([header, ...rows], {
        columns: [{ width: 18 }, { width: 22 }, { width: 50 }, { width: 18 }, { width: 10 }, { width: 14 }, { width: 10 }],
    }).toFile(`${fileName}.xlsx`)

    return products.length
}

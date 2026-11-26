import { useAuth } from "@/auth/AuthContext"
import { ProductSKULink } from "@/components/products/ProductTableHelpers"

/** SKU y nombre; el enlace al detalle solo aparece con permiso para verlo. */
export function ProductCell({ productId, sku, name }: { productId: number; sku: string; name: string }) {
    const { can } = useAuth()

    return (
        <>
            <div className="font-mono text-xs">
                {can("products.view") ? <ProductSKULink productId={productId} sku={sku} /> : sku}
            </div>
            <div className="max-w-72 truncate text-muted-foreground" title={name}>
                {name}
            </div>
        </>
    )
}

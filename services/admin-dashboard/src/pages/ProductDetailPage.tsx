import { PageHeader } from "@/components/PageHeader.tsx"
import {
    NavigationMenu,
    NavigationMenuItem,
    NavigationMenuLink,
    NavigationMenuList,
    navigationMenuTriggerStyle,
} from "@/components/ui/navigation-menu"
import { apiClient } from "@/lib/api"
import { productGeneralSchema } from "@/lib/schemas/product-details"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeftIcon } from "lucide-react"
import { useState } from "react"
import { Link, Outlet, useLocation, useParams } from "react-router-dom"

// Las secciones del detalle. La primera es la ruta índice, así que entrar a
// /products/{id} cae directo en General.
const sections = [
    { label: "General", path: "" },
    { label: "Multimedia", path: "media" },
    { label: "Precios", path: "pricing" },
    { label: "Inventario", path: "inventory" },
    { label: "Números de parte", path: "part-numbers" },
    { label: "Atributos", path: "attributes" },
]

/** Contexto que el layout comparte con cada sección (ver Outlet más abajo). */
export type ProductDetailContext = {
    /** Ruta de la tabla desde la que se abrió el detalle. */
    originPath: string
}

const DEFAULT_ORIGIN = "/products"

/** Etiqueta del botón de regreso, según la tabla de la que se venga. */
function originLabel(originPath: string): string {
    if (originPath.startsWith("/products/pending")) {
        return "Volver a pendientes"
    }
    if (originPath.startsWith("/products/channels")) {
        return "Volver al canal"
    }
    return "Volver a productos"
}

export function ProductDetailPage() {
    const { productId } = useParams<{ productId: string }>()
    const location = useLocation()

    // El origen se captura UNA sola vez, al montar: cambiar de pestaña dentro
    // del detalle es una navegación nueva sin state, así que leerlo en cada
    // render lo perdería en el primer clic. Si se llegó por URL directa o tras
    // recargar (no hay state), cae a la lista de productos.
    const [originPath] = useState(() => {
        const state = location.state as { from?: string } | null
        return state?.from ?? DEFAULT_ORIGIN
    })

    // El encabezado necesita identificar el producto en cualquier sección, así
    // que pide "general" aparte. Comparte queryKey con la sección General, de
    // modo que abrir esa pestaña reutiliza esta respuesta en vez de repetirla.
    const { data: product, isError } = useQuery({
        queryKey: ["product-details", productId, "general"],
        enabled: Boolean(productId),
        queryFn: async () => {
            const response = await apiClient.get(`/api/products/${productId}/details/general`)
            return productGeneralSchema.parse(response.data)
        },
    })

    const basePath = `/products/${productId}`

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <PageHeader>
                <Link
                    to={originPath}
                    aria-label={originLabel(originPath)}
                    title={originLabel(originPath)}
                    className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                >
                    <ArrowLeftIcon className="size-4" />
                </Link>

                <div className="min-w-0">
                    <h1 className="truncate text-sm font-semibold text-foreground">
                        {isError ? "Producto no encontrado" : (product?.sku ?? "Cargando...")}
                    </h1>
                    {product ? (
                        <p className="truncate text-xs text-muted-foreground">{product.name}</p>
                    ) : null}
                </div>

                <NavigationMenu className="ml-2">
                    <NavigationMenuList>
                        {sections.map((section) => {
                            const to = section.path ? `${basePath}/${section.path}` : basePath
                            const isActive = location.pathname === to

                            return (
                                <NavigationMenuItem key={section.label}>
                                    <NavigationMenuLink
                                        active={isActive}
                                        className={navigationMenuTriggerStyle()}
                                        render={<Link to={to} />}
                                    >
                                        {section.label}
                                    </NavigationMenuLink>
                                </NavigationMenuItem>
                            )
                        })}
                    </NavigationMenuList>
                </NavigationMenu>
            </PageHeader>

            <section className="min-h-0 flex-1 overflow-auto p-4">
                <Outlet context={{ originPath } satisfies ProductDetailContext} />
            </section>
        </main>
    )
}

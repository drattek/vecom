import { PageHeader } from "@/components/PageHeader.tsx";
import { TablePagination } from "@/components/TablePagination";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedProductsResponseSchema } from "@/lib/schemas/products";
import { useProductsPaginationStore } from "@/stores/productsPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { Loader2Icon, PlusIcon, SearchIcon, XIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { ChannelBadges, ProductSKULink, ProductThumbnail, SortableTableHead, TableLoadingOverlay } from "@/components/products/ProductTableHelpers";
import { formatPrice } from "@/components/products/ProductTableHelpers";

export function ProductsPage() {
    const offset = useProductsPaginationStore((state) => state.offset)
    const pageSize = useProductsPaginationStore((state) => state.pageSize)
    const sortBy = useProductsPaginationStore((state) => state.sortBy)
    const sortDir = useProductsPaginationStore((state) => state.sortDir)
    const search = useProductsPaginationStore((state) => state.search)
    const setOffset = useProductsPaginationStore((state) => state.setOffset)
    const setPageSize = useProductsPaginationStore((state) => state.setPageSize)
    const setSearch = useProductsPaginationStore((state) => state.setSearch)
    const toggleSort = useProductsPaginationStore((state) => state.toggleSort)

    const [searchInput, setSearchInput] = useState(search)

    useEffect(() => {
        const timer = setTimeout(() => {
            const trimmed = searchInput.trim()
            if (trimmed !== search) {
                setSearch(trimmed)
            }
        }, 300)

        return () => clearTimeout(timer)
    }, [searchInput, search, setSearch])

    const { data, isLoading, isFetching, isError, error } = useQuery({
        queryKey: ["products", offset, pageSize, sortBy, sortDir, search],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/products", {
                params: {
                    offset,
                    pageSize,
                    ...(sortBy ? { sortBy, sortDir } : {}),
                    ...(search ? { search } : {}),
                }
            });

            return paginatedProductsResponseSchema.parse(response.data);
        }
    })

    const products = data?.products ?? []
    const total = data?.total ?? 0

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Productos</p>
            </PageHeader>

            <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
                <div className="flex w-full shrink-0 items-center gap-3">
                    <div className="relative w-full max-w-sm">
                        <SearchIcon className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            value={searchInput}
                            onChange={(event) => setSearchInput(event.target.value)}
                            placeholder="Buscar por SKU, número de parte o nombre"
                            aria-label="Buscar producto"
                            className="px-8"
                        />
                        {searchInput ? (
                            <button
                                type="button"
                                onClick={() => {
                                    setSearchInput("")
                                    setSearch("")
                                }}
                                aria-label="Borrar búsqueda"
                                className="absolute right-1.5 top-1/2 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                            >
                                <XIcon className="size-4" />
                            </button>
                        ) : null}
                    </div>
                    <div className="ml-auto flex items-center gap-3">
                        {isFetching && !isLoading ? (
                            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
                                <Loader2Icon className="size-4 animate-spin" />
                                Actualizando...
                            </span>
                        ) : null}
                        <Button>
                            <PlusIcon />
                            Agregar producto
                        </Button>
                    </div>
                </div>

                {isError ? (
                    <p className="mt-2 text-sm text-destructive">
                        Error al obtener los productos: {error instanceof Error ? error.message : "Error desconocido"}
                    </p>
                ) : null}

                <div className="relative mt-4 min-h-0 flex-1">
                    <Table containerClassName="h-full overflow-auto">
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-16"></TableHead>
                                <SortableTableHead label="SKU" column="sku" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <SortableTableHead label="Número de parte" column="partNumber" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <SortableTableHead label="Nombre" column="name" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <SortableTableHead label="Marca" column="brand" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <SortableTableHead label="Stock" column="stock" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <SortableTableHead label="Precio" column="price" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                                <TableHead>Canales</TableHead>
                                <TableHead>Acciones</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading && products.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={9} className="text-center text-muted-foreground">
                                        Cargando productos...
                                    </TableCell>
                                </TableRow>
                            ) : null}

                            {!isLoading && products.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={9} className="text-center text-muted-foreground">
                                        {search
                                            ? "No hay productos que coincidan con la búsqueda."
                                            : "No se encontraron productos."}
                                    </TableCell>
                                </TableRow>
                            ) : null}

                            {products.map((product) => (
                                <TableRow key={product.id}>
                                    <TableCell>
                                        <ProductThumbnail src={product.imageUrl} alt={product.name} />
                                    </TableCell>
                                    <TableCell>
                                        <ProductSKULink productId={product.id} sku={product.sku} />
                                    </TableCell>
                                    <TableCell>{product.partNumber}</TableCell>
                                    <TableCell>{product.name}</TableCell>
                                    <TableCell>{product.brandName ?? "—"}</TableCell>
                                    <TableCell className="tabular-nums">{product.totalStock}</TableCell>
                                    <TableCell className="tabular-nums whitespace-nowrap">
                                        {product.price ? formatPrice(product.price) : "—"}
                                    </TableCell>
                                    <TableCell>
                                        <ChannelBadges channels={product.channels} />
                                    </TableCell>
                                    <TableCell></TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>

                    <TableLoadingOverlay show={isFetching && !isLoading} />
                </div>

                <TablePagination
                    offset={offset}
                    pageSize={pageSize}
                    total={total}
                    onOffsetChange={setOffset}
                    onPageSizeChange={setPageSize}
                />
            </section>
        </main>
    )
}

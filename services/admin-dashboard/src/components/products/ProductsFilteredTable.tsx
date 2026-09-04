import { TablePagination } from "@/components/TablePagination";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedProductsResponseSchema } from "@/lib/schemas/products";
import type { ProductSortColumn, SortDirection } from "@/stores/productsPaginationStore";
import { formatPrice, ProductThumbnail, SortableTableHead } from "@/components/products/ProductTableHelpers";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { PlusIcon, SearchIcon, XIcon } from "lucide-react";
import { useEffect, useState } from "react";

const DEFAULT_PAGE_SIZE = 10

type ProductsFilteredTableProps = {
    /** Identifies this view for the query cache; combined with pagination/sort/search. */
    queryKey: unknown[]
    /** Extra fixed query params sent to GET /api/products (e.g. connectionId, pending). */
    filterParams?: Record<string, string | number | boolean>
    /** Shown in the empty state when there's no active search. */
    emptyMessage: string
}

export function ProductsFilteredTable({ queryKey, filterParams, emptyMessage }: ProductsFilteredTableProps) {
    const [offset, setOffset] = useState(0);
    const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
    const [sortBy, setSortBy] = useState<ProductSortColumn | null>(null);
    const [sortDir, setSortDir] = useState<SortDirection>("asc");
    const [search, setSearch] = useState("");
    const [searchInput, setSearchInput] = useState("");

    useEffect(() => {
        const timer = setTimeout(() => {
            const trimmed = searchInput.trim();
            if (trimmed !== search) {
                setSearch(trimmed);
                setOffset(0);
            }
        }, 300);

        return () => clearTimeout(timer);
    }, [searchInput, search]);

    function toggleSort(column: ProductSortColumn) {
        if (sortBy !== column) {
            setSortBy(column);
            setSortDir("asc");
            setOffset(0);
            return;
        }

        if (sortDir === "asc") {
            setSortDir("desc");
            setOffset(0);
            return;
        }

        setSortBy(null);
        setSortDir("asc");
        setOffset(0);
    }

    const { data, isLoading, isError, error } = useQuery({
        queryKey: [...queryKey, offset, pageSize, sortBy, sortDir, search],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/products", {
                params: {
                    offset,
                    pageSize,
                    ...filterParams,
                    ...(sortBy ? { sortBy, sortDir } : {}),
                    ...(search ? { search } : {}),
                },
            });

            return paginatedProductsResponseSchema.parse(response.data);
        },
    });

    const products = data?.products ?? [];
    const total = data?.total ?? 0;

    return (
        <>
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
                                setSearchInput("");
                                setSearch("");
                            }}
                            aria-label="Borrar búsqueda"
                            className="absolute right-1.5 top-1/2 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                        >
                            <XIcon className="size-4" />
                        </button>
                    ) : null}
                </div>
                <Button className="ml-auto">
                    <PlusIcon />
                    Agregar producto
                </Button>
            </div>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los productos: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead className="w-16"></TableHead>
                        <SortableTableHead label="SKU" column="sku" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <SortableTableHead label="Número de parte" column="partNumber" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <SortableTableHead label="Nombre" column="name" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <SortableTableHead label="Marca" column="brand" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <SortableTableHead label="Stock" column="stock" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <SortableTableHead label="Precio" column="price" sortBy={sortBy} sortDir={sortDir} onSort={toggleSort} />
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && products.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={8} className="text-center text-muted-foreground">
                                Cargando productos...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && products.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={8} className="text-center text-muted-foreground">
                                {search ? "No hay productos que coincidan con la búsqueda." : emptyMessage}
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {products.map((product) => (
                        <TableRow key={product.id}>
                            <TableCell>
                                <ProductThumbnail src={product.imageUrl} alt={product.name} />
                            </TableCell>
                            <TableCell>{product.sku}</TableCell>
                            <TableCell>{product.partNumber}</TableCell>
                            <TableCell>{product.name}</TableCell>
                            <TableCell>{product.brandName ?? "—"}</TableCell>
                            <TableCell className="tabular-nums">{product.totalStock}</TableCell>
                            <TableCell className="tabular-nums whitespace-nowrap">
                                {product.price ? formatPrice(product.price) : "—"}
                            </TableCell>
                            <TableCell></TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={total}
                onOffsetChange={setOffset}
                onPageSizeChange={(nextPageSize) => {
                    setPageSize(nextPageSize);
                    setOffset(0);
                }}
            />
        </>
    );
}

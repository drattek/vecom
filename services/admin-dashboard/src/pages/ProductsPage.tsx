import { PageHeader } from "@/components/PageHeader.tsx";
import { TablePagination } from "@/components/TablePagination";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { apiClient } from "@/lib/api";
import { cn } from "@/lib/utils";
import { paginatedProductsResponseSchema, type ProductChannel, type ProductPrice } from "@/lib/schemas/products";
import {
    useProductsPaginationStore,
    type ProductSortColumn,
    type SortDirection,
} from "@/stores/productsPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon, PlusIcon, SearchIcon, XIcon } from "lucide-react";
import { useEffect, useState } from "react";

function formatPrice(price: ProductPrice): string {
    const amount = Number(price.amount)
    const formatted = Number.isFinite(amount)
        ? amount.toLocaleString("es-MX", { minimumFractionDigits: 2, maximumFractionDigits: 2 })
        : price.amount

    return `${price.currencySymbol}${formatted} ${price.currencyCode}`
}

function ChannelBadges({ channels }: { channels: ProductChannel[] }) {
    if (channels.length === 0) {
        return <span className="text-muted-foreground">—</span>
    }

    return (
        <div className="flex flex-wrap gap-1">
            {channels.map((channel) => (
                <Tooltip key={channel.connectionId}>
                    <TooltipTrigger
                        render={
                            <span
                                tabIndex={0}
                                className="flex h-8 min-w-8 items-center justify-center rounded-md border border-border bg-muted px-1.5 text-xs font-semibold uppercase text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                            >
                                {channel.code}
                            </span>
                        }
                    />
                    <TooltipContent>{channel.name}</TooltipContent>
                </Tooltip>
            ))}
        </div>
    )
}

function ProductThumbnail({ src, alt }: { src?: string | null; alt: string }) {
    const [errored, setErrored] = useState(false)

    if (!src || errored) {
        return <div className="size-10 rounded-md bg-muted" aria-hidden />
    }

    return (
        <img
            src={src}
            alt={alt}
            className="size-10 rounded-md object-cover"
            onError={() => setErrored(true)}
        />
    )
}

function SortableTableHead({
    label,
    column,
    sortBy,
    sortDir,
    onSort,
}: {
    label: string
    column: ProductSortColumn
    sortBy: ProductSortColumn | null
    sortDir: SortDirection
    onSort: (column: ProductSortColumn) => void
}) {
    const isActive = sortBy === column
    const Icon = isActive ? (sortDir === "asc" ? ArrowUpIcon : ArrowDownIcon) : ArrowUpDownIcon

    return (
        <TableHead aria-sort={isActive ? (sortDir === "asc" ? "ascending" : "descending") : "none"}>
            <button
                type="button"
                onClick={() => onSort(column)}
                className="inline-flex items-center gap-1 font-medium outline-none hover:text-foreground focus-visible:underline"
            >
                {label}
                <Icon className={cn("size-3.5", isActive ? "text-foreground" : "text-muted-foreground")} />
            </button>
        </TableHead>
    )
}

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

    const { data, isLoading, isError, error } = useQuery({
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
                                <TableCell>{product.sku}</TableCell>
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

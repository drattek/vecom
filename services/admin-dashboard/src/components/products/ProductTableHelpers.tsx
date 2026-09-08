import { TableHead } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { ProductChannel, ProductPrice } from "@/lib/schemas/products";
import type { ProductSortColumn, SortDirection } from "@/stores/productsPaginationStore";
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon, Loader2Icon } from "lucide-react";
import { useState } from "react";
import { Link, useLocation } from "react-router-dom";

export function formatPrice(price: ProductPrice): string {
    const amount = Number(price.amount)
    const formatted = Number.isFinite(amount)
        ? amount.toLocaleString("es-MX", { minimumFractionDigits: 2, maximumFractionDigits: 2 })
        : price.amount

    return `${price.currencySymbol}${formatted} ${price.currencyCode}`
}

export function ChannelBadges({ channels }: { channels: ProductChannel[] }) {
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

/**
 * El SKU es el punto de entrada al detalle del producto desde cualquier tabla.
 * Manda en el state la ruta actual (incluyendo query string) para que el botón
 * de regresar del detalle vuelva a la tabla de la que se salió — Todos,
 * Pendientes o la de un canal— en vez de siempre a /products.
 */
export function ProductSKULink({ productId, sku }: { productId: number; sku: string }) {
    const location = useLocation()

    return (
        <Link
            to={`/products/${productId}`}
            state={{ from: `${location.pathname}${location.search}` }}
            className="font-medium underline-offset-2 outline-none hover:underline focus-visible:underline focus-visible:ring-2 focus-visible:ring-ring/50"
        >
            {sku}
        </Link>
    )
}

export function ProductThumbnail({ src, alt }: { src?: string | null; alt: string }) {
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

// TableLoadingOverlay dims the table and blocks pointer events while data is
// refetching (sort/search/page change) — a fetch in flight replaces the row
// data underneath it, so clicks on a row (e.g. opening/editing it, once rows
// become interactive) need to be blocked until the new data lands. Render it
// as an absolutely-positioned sibling of <Table> inside a `relative`
// wrapper, not as a child of <Table> itself — a <table> can't have a <div>
// child.
export function TableLoadingOverlay({ show }: { show: boolean }) {
    if (!show) {
        return null
    }

    return (
        <div className="absolute inset-0 z-20 flex cursor-wait items-center justify-center bg-background/60 backdrop-blur-[1px]">
            <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
        </div>
    )
}

export function SortableTableHead({
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

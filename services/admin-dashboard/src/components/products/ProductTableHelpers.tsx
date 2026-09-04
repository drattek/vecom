import { TableHead } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { ProductChannel, ProductPrice } from "@/lib/schemas/products";
import type { ProductSortColumn, SortDirection } from "@/stores/productsPaginationStore";
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon } from "lucide-react";
import { useState } from "react";

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

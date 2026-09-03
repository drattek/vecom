import type { MouseEvent } from "react"
import { ChevronsLeftIcon, ChevronsRightIcon } from "lucide-react"
import { Field, FieldLabel } from "@/components/ui/field"
import { Pagination, PaginationContent, PaginationItem, PaginationLink, PaginationNext, PaginationPrevious } from "@/components/ui/pagination"
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

type TablePaginationProps = {
    offset: number
    pageSize: number
    total: number
    onOffsetChange: (offset: number) => void
    onPageSizeChange: (pageSize: number) => void
    pageSizeOptions?: number[]
}

export function TablePagination({
    offset,
    pageSize,
    total,
    onOffsetChange,
    onPageSizeChange,
    pageSizeOptions = [10, 25, 50],
}: TablePaginationProps) {
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    const currentPage = Math.min(totalPages, Math.floor(offset / pageSize) + 1)
    const canGoPrevious = offset > 0
    const canGoNext = offset + pageSize < total
    const lastOffset = (totalPages - 1) * pageSize

    const goTo = (target: number) => (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        const next = Math.min(Math.max(0, target), lastOffset)
        if (next !== offset) {
            onOffsetChange(next)
        }
    }

    const handlePageSizeChange = (value: string | null) => {
        if (!value) {
            return
        }

        const parsed = parseInt(value, 10)
        if (Number.isNaN(parsed) || parsed <= 0) {
            return
        }
        onPageSizeChange(parsed)
    }

    const disabledClass = "pointer-events-none opacity-50"

    return (
        <div className="flex shrink-0 items-center justify-end border-t pt-3">
            <div className="flex-1"></div>
            <div className="flex items-center gap-2">
                <Field orientation="horizontal">
                    <FieldLabel htmlFor="rows-per-page">Cantidad:</FieldLabel>
                    <Select value={pageSize.toString()} onValueChange={handlePageSizeChange}>
                        <SelectTrigger id="rows-per-page">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent align="start">
                            <SelectGroup>
                                {pageSizeOptions.map((option) => (
                                    <SelectItem key={option} value={option.toString()}>{option}</SelectItem>
                                ))}
                            </SelectGroup>
                        </SelectContent>
                    </Select>
                </Field>
                <Pagination>
                    <PaginationContent>
                        <PaginationItem>
                            <PaginationLink
                                href="#"
                                aria-label="Ir a la primera página"
                                onClick={goTo(0)}
                                aria-disabled={!canGoPrevious}
                                className={!canGoPrevious ? disabledClass : ""}
                            >
                                <ChevronsLeftIcon />
                            </PaginationLink>
                        </PaginationItem>
                        <PaginationItem>
                            <PaginationPrevious
                                href="#"
                                onClick={goTo(offset - pageSize)}
                                aria-disabled={!canGoPrevious}
                                className={!canGoPrevious ? disabledClass : ""}
                            >
                                Previous
                            </PaginationPrevious>
                        </PaginationItem>
                        <PaginationItem>
                            <PaginationLink href="#" isActive>{currentPage}</PaginationLink>
                        </PaginationItem>
                        <PaginationItem>
                            <PaginationNext
                                href="#"
                                onClick={goTo(offset + pageSize)}
                                aria-disabled={!canGoNext}
                                className={!canGoNext ? disabledClass : ""}
                            >
                                Next
                            </PaginationNext>
                        </PaginationItem>
                        <PaginationItem>
                            <PaginationLink
                                href="#"
                                aria-label="Ir a la última página"
                                onClick={goTo(lastOffset)}
                                aria-disabled={!canGoNext}
                                className={!canGoNext ? disabledClass : ""}
                            >
                                <ChevronsRightIcon />
                            </PaginationLink>
                        </PaginationItem>
                    </PaginationContent>
                </Pagination>
            </div>
        </div>
    )
}

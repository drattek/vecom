import { paginatedFilesResponseSchema } from '@/lib/schemas/files'
import { useFilePaginationStore } from '@/stores/filePaginationStore'
import { useQuery } from '@tanstack/react-query'
import type { MouseEvent } from 'react'
import { apiClient } from '@/lib/api'
import { Field, FieldLabel } from '@/components/ui/field'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Pagination, PaginationContent, PaginationItem, PaginationLink, PaginationNext, PaginationPrevious } from '@/components/ui/pagination'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function FilesPage() {
    const offset = useFilePaginationStore((state) => state.offset)
    const pageSize = useFilePaginationStore((state) => state.pageSize)
    const setOffset = useFilePaginationStore((state) => state.setOffset)
    const setPageSize = useFilePaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ['files', offset, pageSize],
        queryFn: async () => {
            const response = await apiClient.get('/api/files', {
                params: {
                    offset,
                    pageSize,
                },
            })

            return paginatedFilesResponseSchema.parse(response.data)
        },
    })

    const files = data?.files ?? []
    const total = data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    const currentPage = Math.min(totalPages, Math.floor(offset / pageSize) + 1)
    const canGoPrevious = offset > 0
    const canGoNext = offset + pageSize < total

    const handlePageSizeChange = (value: string | null) => {
        if (!value) {
            return
        }

        const parsed = parseInt(value, 10)
        if (Number.isNaN(parsed) || parsed <= 0) {
            return
        }
        setPageSize(parsed)
    }

    const goToPreviousPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        if (!canGoPrevious) {
            return
        }

        setOffset(Math.max(0, offset - pageSize))
    }

    const goToNextPage = (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        if (!canGoNext) {
            return
        }

        setOffset(offset + pageSize)
    }

    return (
        <section className="p-4">
            <h2 className="text-xl font-semibold text-foreground">Files</h2>
            <p className="mt-2 text-sm text-muted-foreground">Manage files</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error loading files: {error instanceof Error ? error.message : 'Unknown error'}
                </p>
            ) : null}

            <div className="flex items-center justify-end">
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
                                    <SelectItem value="5">5</SelectItem>
                                    <SelectItem value="10">10</SelectItem>
                                    <SelectItem value="20">20</SelectItem>
                                </SelectGroup>
                            </SelectContent>
                        </Select>
                    </Field>
                    <Pagination>
                        <PaginationContent>
                            <PaginationItem>
                                <PaginationPrevious
                                    href="#"
                                    onClick={goToPreviousPage}
                                    aria-disabled={!canGoPrevious}
                                    className={!canGoPrevious ? 'pointer-events-none opacity-50' : ''}
                                >
                                    Previous
                                </PaginationPrevious>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationLink href="#">1</PaginationLink>
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationNext
                                    href="#"
                                    onClick={goToNextPage}
                                    aria-disabled={!canGoNext}
                                    className={!canGoNext ? 'pointer-events-none opacity-50' : ''}
                                >
                                    Next
                                </PaginationNext>
                            </PaginationItem>
                        </PaginationContent>
                    </Pagination>
                </div>
            </div>

            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>Tipo</TableHead>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Tamaño</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && files.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center">
                                Loading...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && files.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={4} className="text-center">
                                No files found
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {files.map((file) => (
                        <TableRow key={file.id}>
                            <TableCell>{file.fileType}</TableCell>
                            <TableCell>{file.filename}</TableCell>
                            <TableCell>{file.size}</TableCell>
                            <TableCell> </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </section>
    )
}
import { paginatedFilesResponseSchema } from '@/lib/schemas/files'
import { useFilePaginationStore } from '@/stores/filePaginationStore'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { TablePagination } from '@/components/TablePagination'
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

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Files</h2>
            <p className="mt-2 text-sm text-muted-foreground">Manage files</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error loading files: {error instanceof Error ? error.message : 'Unknown error'}
                </p>
            ) : null}

            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
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

            <TablePagination
                offset={offset}
                pageSize={pageSize}
                total={total}
                onOffsetChange={setOffset}
                onPageSizeChange={setPageSize}
                pageSizeOptions={[5, 10, 20]}
            />
        </section>
    )
}
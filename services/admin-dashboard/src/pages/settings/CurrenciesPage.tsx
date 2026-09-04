import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedCurrenciesResponseSchema } from "@/lib/schemas/currencies";
import { useCurrenciesPaginationStore } from "@/stores/currenciesPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

export function CurrenciesPage() {
    const offset = useCurrenciesPaginationStore((state) => state.offset)
    const pageSize = useCurrenciesPaginationStore((state) => state.pageSize)
    const setOffset = useCurrenciesPaginationStore((state) => state.setOffset)
    const setPageSize = useCurrenciesPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["currencies", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/currencies", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedCurrenciesResponseSchema.parse(response.data);
        }
    })

    const currencies = data?.currencies ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Divisas</h2>
            <p className="mt-2 text-sm text-muted-foreground">Mantiene las divisas disponibles para precios, compras y reportes.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las divisas: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Código</TableHead>
                        <TableHead>Símbolo</TableHead>
                        <TableHead>Decimales</TableHead>
                        <TableHead>Fecha de creación</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && currencies.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                Cargando divisas...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && currencies.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                No se encontraron divisas.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {currencies.map((currency) => (
                        <TableRow key={currency.id}>
                            <TableCell>{currency.name}</TableCell>
                            <TableCell>{currency.code}</TableCell>
                            <TableCell>{currency.symbol}</TableCell>
                            <TableCell className="tabular-nums">{currency.decimalPlaces}</TableCell>
                            <TableCell>{formatDate(currency.createdAt)}</TableCell>
                            <TableCell>{formatDate(currency.updatedAt)}</TableCell>
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
    )
}

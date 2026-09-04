import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedExchangeRatesResponseSchema, type ExchangeRate } from "@/lib/schemas/exchange-rates";
import { useExchangeRatesPaginationStore } from "@/stores/exchangeRatesPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

function formatRate(rate: ExchangeRate): string {
    const amount = Number(rate.rate)
    const formatted = Number.isFinite(amount)
        ? amount.toLocaleString("es-MX", { minimumFractionDigits: 2, maximumFractionDigits: 6 })
        : rate.rate

    const from = rate.fromCurrencyCode ?? `#${rate.fromCurrencyId}`
    const to = rate.toCurrencyCode ?? `#${rate.toCurrencyId}`

    return `1 ${from} = ${formatted} ${to}`
}

export function ExchangeRatesPage() {
    const offset = useExchangeRatesPaginationStore((state) => state.offset)
    const pageSize = useExchangeRatesPaginationStore((state) => state.pageSize)
    const setOffset = useExchangeRatesPaginationStore((state) => state.setOffset)
    const setPageSize = useExchangeRatesPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["exchange-rates", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/exchange-rates", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedExchangeRatesResponseSchema.parse(response.data);
        }
    })

    const rates = data?.data ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Tipo de cambio</h2>
            <p className="mt-2 text-sm text-muted-foreground">Actualiza y consulta tipos de cambio para conversiones monetarias.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener los tipos de cambio: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Moneda origen</TableHead>
                        <TableHead>Moneda destino</TableHead>
                        <TableHead>Tipo de cambio</TableHead>
                        <TableHead>Última actualización</TableHead>
                        <TableHead>Acciones</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && rates.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                Cargando tipos de cambio...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && rates.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={5} className="text-center text-muted-foreground">
                                No se encontraron tipos de cambio.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {rates.map((rate) => (
                        <TableRow key={rate.id}>
                            <TableCell>
                                {rate.fromCurrencyCode ?? `#${rate.fromCurrencyId}`}
                                {rate.fromCurrencySymbol ? ` (${rate.fromCurrencySymbol})` : ""}
                            </TableCell>
                            <TableCell>
                                {rate.toCurrencyCode ?? `#${rate.toCurrencyId}`}
                                {rate.toCurrencySymbol ? ` (${rate.toCurrencySymbol})` : ""}
                            </TableCell>
                            <TableCell className="tabular-nums whitespace-nowrap">{formatRate(rate)}</TableCell>
                            <TableCell>{formatDate(rate.updatedAt)}</TableCell>
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

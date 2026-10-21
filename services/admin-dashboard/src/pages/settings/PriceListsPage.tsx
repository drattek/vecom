import { TablePagination } from "@/components/TablePagination";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api";
import { paginatedCurrenciesResponseSchema } from "@/lib/schemas/currencies";
import { paginatedPriceListsResponseSchema } from "@/lib/schemas/price-lists";
import { usePriceListsPaginationStore } from "@/stores/priceListsPaginationStore";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

function formatDate(value: string): string {
    const parsed = new Date(value)
    if (Number.isNaN(parsed.getTime())) {
        return "-"
    }
    return parsed.toLocaleString("es-MX", { dateStyle: "medium", timeStyle: "short" })
}

// valid_from/valid_to son DATE ("YYYY-MM-DD"): se formatean sin pasar por Date para evitar corrimientos de zona horaria.
function formatDay(value: string): string {
    const [year, month, day] = value.split("-")
    if (!year || !month || !day) {
        return "-"
    }
    return `${day}/${month}/${year}`
}

export function PriceListsPage() {
    const offset = usePriceListsPaginationStore((state) => state.offset)
    const pageSize = usePriceListsPaginationStore((state) => state.pageSize)
    const setOffset = usePriceListsPaginationStore((state) => state.setOffset)
    const setPageSize = usePriceListsPaginationStore((state) => state.setPageSize)

    const { data, isLoading, isError, error } = useQuery({
        queryKey: ["price-lists", offset, pageSize],
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/price-lists", {
                params: {
                    offset,
                    pageSize
                }
            });

            return paginatedPriceListsResponseSchema.parse(response.data);
        }
    })

    const { data: currenciesData } = useQuery({
        queryKey: ["currencies", "lookup"],
        staleTime: 5 * 60 * 1000,
        queryFn: async () => {
            const response = await apiClient.get("/api/currencies", {
                params: { offset: 0, pageSize: 100 }
            });

            return paginatedCurrenciesResponseSchema.parse(response.data);
        }
    })

    const currencyCodeById = new Map((currenciesData?.currencies ?? []).map((currency) => [currency.id, currency.code]))

    const priceLists = data?.data ?? []
    const total = data?.total ?? 0

    return (
        <section className="flex min-h-0 flex-1 flex-col overflow-hidden p-4">
            <h2 className="text-xl font-semibold text-foreground">Listas de precios</h2>
            <p className="mt-2 text-sm text-muted-foreground">Consulta las listas de precios, su divisa, prioridad y vigencia.</p>

            {isError ? (
                <p className="mt-2 text-sm text-destructive">
                    Error al obtener las listas de precios: {error instanceof Error ? error.message : "Error desconocido"}
                </p>
            ) : null}
            <Table containerClassName="mt-4 min-h-0 flex-1 overflow-auto">
                <TableHeader>
                    <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Divisa</TableHead>
                        <TableHead>Prioridad</TableHead>
                        <TableHead>Estatus</TableHead>
                        <TableHead>Vigencia desde</TableHead>
                        <TableHead>Vigencia hasta</TableHead>
                        <TableHead>Última actualización</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {isLoading && priceLists.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                Cargando listas de precios...
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {!isLoading && priceLists.length === 0 ? (
                        <TableRow>
                            <TableCell colSpan={7} className="text-center text-muted-foreground">
                                No se encontraron listas de precios.
                            </TableCell>
                        </TableRow>
                    ) : null}

                    {priceLists.map((priceList) => (
                        <TableRow key={priceList.id}>
                            <TableCell>{priceList.name}</TableCell>
                            <TableCell>{currencyCodeById.get(priceList.currency) ?? priceList.currency}</TableCell>
                            <TableCell className="tabular-nums">{priceList.priority}</TableCell>
                            <TableCell>{priceList.status}</TableCell>
                            <TableCell>{formatDay(priceList.validFrom)}</TableCell>
                            <TableCell>{formatDay(priceList.validTo)}</TableCell>
                            <TableCell>{formatDate(priceList.updatedAt)}</TableCell>
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

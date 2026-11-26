import type { ReactNode } from "react"
import { ArrowLeftIcon, TriangleAlertIcon } from "lucide-react"
import { Link, useLocation, useParams } from "react-router-dom"

import { useAuth } from "@/auth/AuthContext"
import { PageHeader } from "@/components/PageHeader"
import { StatusBadge } from "@/components/sales/StatusBadge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"
import { formatDateTime } from "@/lib/dashboard"
import { formatAmount, ORDER_STATUS, PAYMENT_STATUS, SHIPPING_STATUS, useSale } from "@/lib/sales"
import type { SaleDetail } from "@/lib/schemas/sales"

const CHARGE_LABEL: Record<string, string> = {
    discount: "Descuento",
    shipping: "Envío",
    tax: "Impuestos",
    fee: "Comisión",
}

const PAID_BY_LABEL: Record<string, string> = {
    buyer: "Comprador",
    seller: "Vendedor",
    marketplace: "Plataforma",
}

function Card({ title, children }: { title: string; children: ReactNode }) {
    return (
        <section className="rounded-lg border border-border p-3">
            <h3 className="mb-2 text-sm font-semibold">{title}</h3>
            {children}
        </section>
    )
}

/** Lista de datos; omite los que la plataforma no entregó. */
function Fields({ rows, empty }: { rows: Array<[string, ReactNode]>; empty: string }) {
    const present = rows.filter(([, value]) => value !== null && value !== undefined && value !== "")
    if (present.length === 0) return <p className="text-sm text-muted-foreground">{empty}</p>
    return (
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
            {present.map(([label, value]) => (
                <div key={label} className="contents">
                    <dt className="text-muted-foreground">{label}</dt>
                    <dd className="min-w-0 break-words">{value}</dd>
                </div>
            ))}
        </dl>
    )
}

function Totals({ sale }: { sale: SaleDetail }) {
    const money = (value: number | null) => formatAmount(value, sale.currencyCode)
    const rows: Array<[string, number | null]> = [
        ["Productos", sale.subtotalAmount],
        ["Descuentos", sale.discountAmount],
        ["Envío pagado por el comprador", sale.shippingAmount],
        ["Impuestos", sale.taxAmount],
        ["Total de la venta", sale.totalAmount],
        ["Pagado", sale.paidAmount],
        ["Comisión de la plataforma", sale.marketplaceFeeAmount],
        ["Envío a cargo del vendedor", sale.shippingCostSeller],
        ["Neto estimado", sale.estimatedNetAmount],
    ]

    return (
        <>
            <dl className="space-y-1 text-sm">
                {rows.map(([label, value]) => (
                    <div key={label} className="flex justify-between gap-3">
                        <dt className="text-muted-foreground">{label}</dt>
                        <dd className="whitespace-nowrap tabular-nums">{money(value)}</dd>
                    </div>
                ))}
            </dl>
            <p className="mt-2 text-xs text-muted-foreground">
                "—" indica que no se tiene el dato. El neto es un estimado (total menos comisión y envío del vendedor), no
                una conciliación de pagos.
                {sale.shippingCostOnRelatedSale
                    ? " El envío es compartido con otra orden del paquete y su costo se resta solo allí."
                    : ""}
            </p>
            {sale.charges.length > 0 ? (
                <ul className="mt-3 space-y-1 border-t pt-2 text-xs">
                    {sale.charges.map((charge, index) => (
                        <li key={index} className="flex justify-between gap-3">
                            <span>
                                {CHARGE_LABEL[charge.type] ?? charge.type}
                                {charge.description ? `: ${charge.description}` : ""}
                                {charge.paidBy ? (
                                    <span className="text-muted-foreground"> · {PAID_BY_LABEL[charge.paidBy] ?? charge.paidBy}</span>
                                ) : null}
                            </span>
                            <span className="whitespace-nowrap tabular-nums">{money(charge.amount)}</span>
                        </li>
                    ))}
                </ul>
            ) : null}
        </>
    )
}

function Items({ sale }: { sale: SaleDetail }) {
    const { can } = useAuth()
    const money = (value: number | null) => formatAmount(value, sale.currencyCode)

    if (sale.items.length === 0) {
        return <p className="text-sm text-muted-foreground">No se obtuvieron los productos de esta venta.</p>
    }
    return (
        <Table containerClassName="overflow-x-auto">
            <TableHeader>
                <TableRow>
                    <TableHead>Producto vendido</TableHead>
                    <TableHead>Catálogo</TableHead>
                    <TableHead className="text-right">Cantidad</TableHead>
                    <TableHead className="text-right">Precio unitario</TableHead>
                    <TableHead className="text-right">Importe</TableHead>
                    <TableHead className="text-right">Comisión</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {sale.items.map((item) => (
                    <TableRow key={item.id}>
                        <TableCell className="min-w-56 whitespace-normal">
                            {item.title ?? "Sin título"}
                            <div className="text-xs text-muted-foreground">
                                {item.sku ? `SKU ${item.sku}` : "Sin SKU"}
                                {item.externalProductId ? ` · ID ${item.externalProductId}` : ""}
                                {item.externalVariationId ? ` · Variación ${item.externalVariationId}` : ""}
                            </div>
                        </TableCell>
                        <TableCell className="min-w-40 whitespace-normal">
                            {item.productId === null ? (
                                <span className="text-amber-700 dark:text-amber-400">Sin vincular</span>
                            ) : can("products.view") ? (
                                <Link to={`/products/${item.productId}`} className="text-primary underline-offset-2 hover:underline">
                                    {item.productName ?? "Ver producto"}
                                </Link>
                            ) : (
                                (item.productName ?? "Vinculado")
                            )}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{item.quantity.toLocaleString("es-MX")}</TableCell>
                        <TableCell className="text-right whitespace-nowrap tabular-nums">
                            {money(item.unitPrice)}
                            {item.listUnitPrice !== null && item.unitPrice !== null && item.listUnitPrice - item.unitPrice > 0.004 ? (
                                <div className="text-xs text-muted-foreground line-through">{money(item.listUnitPrice)}</div>
                            ) : null}
                        </TableCell>
                        <TableCell className="text-right whitespace-nowrap tabular-nums">{money(item.totalAmount)}</TableCell>
                        <TableCell className="text-right whitespace-nowrap tabular-nums">{money(item.feeAmount)}</TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    )
}

function SaleBody({ sale }: { sale: SaleDetail }) {
    const money = (value: number | null) => formatAmount(value, sale.currencyCode)
    const optionalDate = (value: string | null) => (value ? formatDateTime(value) : null)

    return (
        <div className="space-y-3">
            <div>
                <h2 className="text-xl font-semibold text-foreground">Venta {sale.orderNumber ?? sale.externalId}</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                    {sale.channelName} · {sale.connectionName} · {formatDateTime(sale.orderedAt)}
                </p>
                <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    Venta <StatusBadge statuses={ORDER_STATUS} value={sale.orderStatus} />
                    Pago <StatusBadge statuses={PAYMENT_STATUS} value={sale.paymentStatus} />
                    Envío <StatusBadge statuses={SHIPPING_STATUS} value={sale.shippingStatus} />
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                    Estado en la plataforma: {sale.externalStatus ?? "—"}
                    {sale.externalStatusDetail ? ` (${sale.externalStatusDetail})` : ""} · Sincronizada el{" "}
                    {formatDateTime(sale.lastSyncedAt)}
                </p>
            </div>

            {sale.isIncomplete ? (
                <div className="flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                    <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
                    <p className="min-w-0 break-words">
                        Faltan datos de esta venta; se volverán a consultar. {sale.incompleteReason}
                    </p>
                </div>
            ) : null}

            {sale.relatedSales.length > 0 ? (
                <p className="text-sm">
                    Parte del paquete {sale.packId}. Otras órdenes:{" "}
                    {sale.relatedSales.map((related, index) => (
                        <span key={related.id}>
                            {index > 0 ? ", " : ""}
                            <Link to={`/sales/${related.id}`} className="text-primary underline-offset-2 hover:underline">
                                {related.externalId}
                            </Link>{" "}
                            ({money(related.totalAmount)})
                        </span>
                    ))}
                </p>
            ) : null}

            <Card title="Productos">
                <Items sale={sale} />
            </Card>

            <div className="grid gap-3 lg:grid-cols-2">
                <Card title="Desglose del precio">
                    <Totals sale={sale} />
                </Card>

                <div className="space-y-3">
                    <Card title="Cliente">
                        <Fields
                            empty="La plataforma no entregó datos del cliente."
                            rows={[
                                ["Nombre", sale.customerName],
                                ["Usuario", sale.customerNickname],
                                ["Correo", sale.customerEmail],
                                ["Teléfono", sale.customerPhone],
                                ["ID en la plataforma", sale.customerExternalId],
                            ]}
                        />
                    </Card>
                    <Card title="Facturación">
                        <Fields
                            empty="Sin datos fiscales."
                            rows={[
                                ["RFC", sale.taxId],
                                ["Razón social", sale.taxName],
                                ["Régimen", sale.taxRegime],
                                ["Uso de CFDI", sale.cfdiUse],
                                ["Dirección", sale.billingAddress],
                            ]}
                        />
                    </Card>
                </div>

                <Card title="Pagos">
                    {sale.payments.length === 0 ? (
                        <p className="text-sm text-muted-foreground">Sin pagos registrados.</p>
                    ) : (
                        <div className="space-y-3">
                            {sale.payments.map((payment) => (
                                <Fields
                                    key={payment.externalId}
                                    empty=""
                                    rows={[
                                        ["Importe", money(payment.amount)],
                                        ["Método", payment.method],
                                        ["Tipo", payment.paymentType],
                                        ["Mensualidades", payment.installments],
                                        ["Estado en la plataforma", payment.externalStatus],
                                        ["Reembolsado", payment.refundedAmount ? money(payment.refundedAmount) : null],
                                        ["Aprobado", optionalDate(payment.approvedAt)],
                                    ]}
                                />
                            ))}
                        </div>
                    )}
                </Card>

                <Card title="Envío">
                    {sale.shipments.length === 0 ? (
                        <p className="text-sm text-muted-foreground">Sin datos de envío.</p>
                    ) : (
                        <div className="space-y-3">
                            {sale.shipments.map((shipment) => (
                                <Fields
                                    key={shipment.externalId}
                                    empty="Sin datos de envío."
                                    rows={[
                                        ["Estado en la plataforma", [shipment.externalStatus, shipment.externalSubstatus].filter(Boolean).join(" / ")],
                                        ["Tipo logístico", [shipment.mode, shipment.logisticType].filter(Boolean).join(" / ")],
                                        ["Transportista", shipment.carrier],
                                        ["Guía", shipment.trackingNumber],
                                        ["Recibe", shipment.receiverName],
                                        ["Teléfono", shipment.receiverPhone],
                                        ["Dirección", shipment.receiverAddress],
                                        ["Despachar antes de", optionalDate(shipment.dispatchDeadline)],
                                        ["Entrega estimada", optionalDate(shipment.estimatedDeliveryAt)],
                                        ["Enviado", optionalDate(shipment.shippedAt)],
                                        ["Entregado", optionalDate(shipment.deliveredAt)],
                                    ]}
                                />
                            ))}
                        </div>
                    )}
                </Card>
            </div>
        </div>
    )
}

export function SaleDetailPage() {
    const { saleId } = useParams()
    const location = useLocation()
    const parsedId = Number(saleId)
    const id = Number.isInteger(parsedId) && parsedId > 0 ? parsedId : null
    const sale = useSale(id)
    const listSearch = (location.state as { listSearch?: string } | null)?.listSearch

    return (
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-2">
            <PageHeader>
                <p>Ventas</p>
            </PageHeader>

            <section className="min-h-0 flex-1 overflow-auto p-4">
                <Link
                    to={{ pathname: "/sales", search: listSearch }}
                    className="mb-3 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
                >
                    <ArrowLeftIcon className="size-4" /> Volver a ventas
                </Link>

                {id === null ? <p className="text-sm text-destructive">La venta indicada no es válida.</p> : null}
                {sale.isLoading ? <p className="text-sm text-muted-foreground">Cargando venta...</p> : null}
                {sale.isError ? (
                    <p className="text-sm text-destructive">
                        {getServerErrorMessage(sale.error, "No se pudo consultar la venta.")}
                    </p>
                ) : null}
                {sale.data ? <SaleBody sale={sale.data} /> : null}
            </section>
        </main>
    )
}

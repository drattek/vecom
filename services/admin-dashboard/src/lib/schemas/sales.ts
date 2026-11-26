import { z } from "zod"

// Contrato de /api/sales (ADR 0010). Un importe null es desconocido, no cero.

const text = z.string().nullable()
const amount = z.number().nullable()
const date = z.string().nullable()

export const saleListItemSchema = z.object({
    id: z.number(),
    connectionId: z.number(),
    connectionName: z.string(),
    channelCode: z.string(),
    channelName: z.string(),
    externalId: z.string(),
    orderNumber: text,
    packId: text,
    orderedAt: z.string(),
    orderStatus: z.string(),
    paymentStatus: z.string(),
    shippingStatus: z.string(),
    customerName: text,
    currencyCode: text,
    totalAmount: amount,
    itemCount: z.number(),
    unlinkedItems: z.number(),
    dispatchDeadline: date,
    isIncomplete: z.boolean(),
})

export const paginatedSalesSchema = z.object({
    total: z.number(),
    offset: z.number(),
    pageSize: z.number(),
    sales: z.array(saleListItemSchema),
})

const saleItemSchema = z.object({
    id: z.number(),
    externalProductId: text,
    externalVariationId: text,
    sku: text,
    title: text,
    quantity: z.number(),
    unitPrice: amount,
    listUnitPrice: amount,
    discountAmount: amount,
    taxAmount: amount,
    totalAmount: amount,
    feeAmount: amount,
    productId: z.number().nullable(),
    productName: text,
    matchMethod: text,
})

const saleChargeSchema = z.object({
    type: z.string(),
    paidBy: text,
    amount: z.number(),
    description: text,
})

const salePaymentSchema = z.object({
    externalId: z.string(),
    method: text,
    paymentType: text,
    installments: z.number().nullable(),
    externalStatus: text,
    amount,
    refundedAmount: amount,
    approvedAt: date,
})

const saleShipmentSchema = z.object({
    externalId: z.string(),
    externalStatus: text,
    externalSubstatus: text,
    mode: text,
    logisticType: text,
    carrier: text,
    trackingNumber: text,
    receiverName: text,
    receiverPhone: text,
    receiverAddress: text,
    dispatchDeadline: date,
    estimatedDeliveryAt: date,
    shippedAt: date,
    deliveredAt: date,
    buyerCost: amount,
    sellerCost: amount,
})

export const saleDetailSchema = saleListItemSchema.extend({
    externalStatus: text,
    externalStatusDetail: text,
    closedAt: date,
    externalUpdatedAt: date,
    lastSyncedAt: z.string(),
    incompleteReason: text,
    customerExternalId: text,
    customerNickname: text,
    customerEmail: text,
    customerPhone: text,
    taxId: text,
    taxName: text,
    taxRegime: text,
    cfdiUse: text,
    billingAddress: text,
    subtotalAmount: amount,
    discountAmount: amount,
    shippingAmount: amount,
    taxAmount: amount,
    paidAmount: amount,
    marketplaceFeeAmount: amount,
    shippingCostSeller: amount,
    items: z.array(saleItemSchema),
    charges: z.array(saleChargeSchema),
    payments: z.array(salePaymentSchema),
    shipments: z.array(saleShipmentSchema),
    relatedSales: z.array(z.object({ id: z.number(), externalId: z.string(), totalAmount: amount })),
    estimatedNetAmount: amount,
    shippingCostOnRelatedSale: z.boolean(),
})

export const salesSyncConnectionSchema = z.object({
    connectionId: z.number(),
    connectionName: z.string(),
    channelCode: z.string(),
    channelName: z.string(),
    active: z.boolean(),
    running: z.boolean(),
    initialLoadDone: z.boolean(),
    lastStartedAt: date,
    lastFinishedAt: date,
    lastSuccessAt: date,
    lastError: text,
    lastSavedCount: z.number(),
    lastFailedCount: z.number(),
})

export const salesSyncStatusSchema = z.object({
    enabled: z.boolean(),
    intervalMinutes: z.number(),
    connections: z.array(salesSyncConnectionSchema),
})

export type SaleListItem = z.infer<typeof saleListItemSchema>
export type SaleDetail = z.infer<typeof saleDetailSchema>
export type SalesSyncConnection = z.infer<typeof salesSyncConnectionSchema>

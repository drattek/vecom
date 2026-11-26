package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Lecturas de la vista "Ventas" (ADR 0010). Las respuestas originales
// (raw_payload) nunca se seleccionan: no se exponen por la API.

type SalesFilter struct {
	ChannelCode    string
	ConnectionID   int64
	OrderStatus    string
	PaymentStatus  string
	ShippingStatus string
	From           *time.Time // incluido
	Until          *time.Time // excluido
	Search         string
	Offset         int
	PageSize       int
}

type SaleListItemDTO struct {
	ID               int64      `json:"id"`
	ConnectionID     int64      `json:"connectionId"`
	ConnectionName   string     `json:"connectionName"`
	ChannelCode      string     `json:"channelCode"`
	ChannelName      string     `json:"channelName"`
	ExternalID       string     `json:"externalId"`
	OrderNumber      *string    `json:"orderNumber"`
	PackID           *string    `json:"packId"`
	OrderedAt        time.Time  `json:"orderedAt"`
	OrderStatus      string     `json:"orderStatus"`
	PaymentStatus    string     `json:"paymentStatus"`
	ShippingStatus   string     `json:"shippingStatus"`
	CustomerName     *string    `json:"customerName"`
	CurrencyCode     *string    `json:"currencyCode"`
	TotalAmount      *float64   `json:"totalAmount"`
	ItemCount        int        `json:"itemCount"`
	UnlinkedItems    int        `json:"unlinkedItems"`
	DispatchDeadline *time.Time `json:"dispatchDeadline"`
	IsIncomplete     bool       `json:"isIncomplete"`
}

type SaleDetailDTO struct {
	SaleListItemDTO
	ExternalStatus       *string    `json:"externalStatus"`
	ExternalStatusDetail *string    `json:"externalStatusDetail"`
	ClosedAt             *time.Time `json:"closedAt"`
	ExternalUpdatedAt    *time.Time `json:"externalUpdatedAt"`
	LastSyncedAt         time.Time  `json:"lastSyncedAt"`
	IncompleteReason     *string    `json:"incompleteReason"`

	CustomerExternalID *string `json:"customerExternalId"`
	CustomerNickname   *string `json:"customerNickname"`
	CustomerEmail      *string `json:"customerEmail"`
	CustomerPhone      *string `json:"customerPhone"`
	TaxID              *string `json:"taxId"`
	TaxName            *string `json:"taxName"`
	TaxRegime          *string `json:"taxRegime"`
	CFDIUse            *string `json:"cfdiUse"`
	BillingAddress     *string `json:"billingAddress"`

	SubtotalAmount       *float64 `json:"subtotalAmount"`
	DiscountAmount       *float64 `json:"discountAmount"`
	ShippingAmount       *float64 `json:"shippingAmount"`
	TaxAmount            *float64 `json:"taxAmount"`
	PaidAmount           *float64 `json:"paidAmount"`
	MarketplaceFeeAmount *float64 `json:"marketplaceFeeAmount"`
	ShippingCostSeller   *float64 `json:"shippingCostSeller"`

	Items     []SaleItemDTO     `json:"items"`
	Charges   []SaleChargeDTO   `json:"charges"`
	Payments  []SalePaymentDTO  `json:"payments"`
	Shipments []SaleShipmentDTO `json:"shipments"`
}

type SaleItemDTO struct {
	ID                  int64    `json:"id"`
	ExternalProductID   *string  `json:"externalProductId"`
	ExternalVariationID *string  `json:"externalVariationId"`
	SKU                 *string  `json:"sku"`
	Title               *string  `json:"title"`
	Quantity            float64  `json:"quantity"`
	UnitPrice           *float64 `json:"unitPrice"`
	ListUnitPrice       *float64 `json:"listUnitPrice"`
	DiscountAmount      *float64 `json:"discountAmount"`
	TaxAmount           *float64 `json:"taxAmount"`
	TotalAmount         *float64 `json:"totalAmount"`
	FeeAmount           *float64 `json:"feeAmount"`
	ProductID           *int64   `json:"productId"`
	ProductName         *string  `json:"productName"`
	MatchMethod         *string  `json:"matchMethod"`
}

type SaleChargeDTO struct {
	Type        string  `json:"type"`
	PaidBy      *string `json:"paidBy"`
	Amount      float64 `json:"amount"`
	Description *string `json:"description"`
}

type SalePaymentDTO struct {
	ExternalID     string     `json:"externalId"`
	Method         *string    `json:"method"`
	PaymentType    *string    `json:"paymentType"`
	Installments   *int       `json:"installments"`
	ExternalStatus *string    `json:"externalStatus"`
	Amount         *float64   `json:"amount"`
	RefundedAmount *float64   `json:"refundedAmount"`
	ApprovedAt     *time.Time `json:"approvedAt"`
}

type SaleShipmentDTO struct {
	ExternalID          string     `json:"externalId"`
	ExternalStatus      *string    `json:"externalStatus"`
	ExternalSubstatus   *string    `json:"externalSubstatus"`
	Mode                *string    `json:"mode"`
	LogisticType        *string    `json:"logisticType"`
	Carrier             *string    `json:"carrier"`
	TrackingNumber      *string    `json:"trackingNumber"`
	ReceiverName        *string    `json:"receiverName"`
	ReceiverPhone       *string    `json:"receiverPhone"`
	ReceiverAddress     *string    `json:"receiverAddress"`
	DispatchDeadline    *time.Time `json:"dispatchDeadline"`
	EstimatedDeliveryAt *time.Time `json:"estimatedDeliveryAt"`
	ShippedAt           *time.Time `json:"shippedAt"`
	DeliveredAt         *time.Time `json:"deliveredAt"`
	BuyerCost           *float64   `json:"buyerCost"`
	SellerCost          *float64   `json:"sellerCost"`
}

// RelatedSaleDTO es otra orden de la misma compra (mismo pack_id).
type RelatedSaleDTO struct {
	ID          int64    `json:"id"`
	ExternalID  string   `json:"externalId"`
	TotalAmount *float64 `json:"totalAmount"`
}

type SalesViewRepository struct {
	db Querier
}

func NewSalesViewRepository(db Querier) *SalesViewRepository {
	return &SalesViewRepository{db: db}
}

const saleListColumns = `
	s.id, s.connection_id, cc.name, ch.code, ch.name,
	s.external_id, s.order_number, s.pack_id, s.ordered_at,
	s.order_status, s.payment_status, s.shipping_status,
	COALESCE(s.customer_name, s.customer_nickname, s.tax_name), s.currency_code, s.total_amount,
	(SELECT COUNT(*) FROM ecom_sale_items i WHERE i.sale_id = s.id AND i.deleted_at IS NULL),
	(SELECT COUNT(*) FROM ecom_sale_items i WHERE i.sale_id = s.id AND i.deleted_at IS NULL AND i.product_id IS NULL),
	(SELECT MIN(sh.dispatch_deadline) FROM ecom_sale_shipments sh WHERE sh.sale_id = s.id AND sh.deleted_at IS NULL),
	s.is_incomplete`

const saleFromClause = `
	FROM ecom_sales s
	JOIN ecom_channel_connections cc ON cc.id = s.connection_id
	JOIN ecom_channels ch ON ch.id = cc.channel_id`

func saleListTargets(item *SaleListItemDTO, deadline *sql.NullTime) []any {
	return []any{
		&item.ID, &item.ConnectionID, &item.ConnectionName, &item.ChannelCode, &item.ChannelName,
		&item.ExternalID, &item.OrderNumber, &item.PackID, &item.OrderedAt,
		&item.OrderStatus, &item.PaymentStatus, &item.ShippingStatus,
		&item.CustomerName, &item.CurrencyCode, &item.TotalAmount,
		&item.ItemCount, &item.UnlinkedItems, deadline, &item.IsIncomplete,
	}
}

func escapeLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func buildSalesWhere(filter SalesFilter) (string, []any) {
	conditions := []string{"s.deleted_at IS NULL"}
	args := []any{}
	add := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filter.ChannelCode != "" {
		add("UPPER(TRIM(ch.code)) = UPPER(?)", filter.ChannelCode)
	}
	if filter.ConnectionID > 0 {
		add("s.connection_id = ?", filter.ConnectionID)
	}
	if filter.OrderStatus != "" {
		add("s.order_status = ?", filter.OrderStatus)
	}
	if filter.PaymentStatus != "" {
		add("s.payment_status = ?", filter.PaymentStatus)
	}
	if filter.ShippingStatus != "" {
		add("s.shipping_status = ?", filter.ShippingStatus)
	}
	if filter.From != nil {
		add("s.ordered_at >= ?", *filter.From)
	}
	if filter.Until != nil {
		add("s.ordered_at < ?", *filter.Until)
	}
	if filter.Search != "" {
		like := "%" + escapeLikePattern(filter.Search) + "%"
		add(`(s.external_id LIKE ? OR s.order_number LIKE ? OR s.pack_id LIKE ? OR s.customer_name LIKE ? OR s.customer_nickname LIKE ?
			OR EXISTS (SELECT 1 FROM ecom_sale_items si WHERE si.sale_id = s.id AND si.deleted_at IS NULL AND si.sku LIKE ?))`,
			like, like, like, like, like, like)
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (r *SalesViewRepository) FindPaginated(ctx context.Context, filter SalesFilter) ([]SaleListItemDTO, int64, error) {
	where, args := buildSalesWhere(filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*)"+saleFromClause+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("error counting sales: %w", err)
	}

	query := "SELECT " + saleListColumns + saleFromClause + where + " ORDER BY s.ordered_at DESC, s.id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, filter.PageSize, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying sales: %w", err)
	}
	defer rows.Close()

	sales := make([]SaleListItemDTO, 0)
	for rows.Next() {
		var item SaleListItemDTO
		var deadline sql.NullTime
		if err := rows.Scan(saleListTargets(&item, &deadline)...); err != nil {
			return nil, 0, fmt.Errorf("error scanning sale: %w", err)
		}
		if deadline.Valid {
			item.DispatchDeadline = &deadline.Time
		}
		sales = append(sales, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating sales: %w", err)
	}
	return sales, total, nil
}

func (r *SalesViewRepository) FindByID(ctx context.Context, id int64) (*SaleDetailDTO, error) {
	query := "SELECT " + saleListColumns + `,
		s.external_status, s.external_status_detail, s.closed_at, s.external_updated_at, s.last_synced_at, s.incomplete_reason,
		s.customer_external_id, s.customer_nickname, s.customer_email, s.customer_phone,
		s.tax_id, s.tax_name, s.tax_regime, s.cfdi_use, s.billing_address,
		s.subtotal_amount, s.discount_amount, s.shipping_amount, s.tax_amount, s.paid_amount,
		s.marketplace_fee_amount, s.shipping_cost_seller` + saleFromClause + " WHERE s.id = ? AND s.deleted_at IS NULL"

	var sale SaleDetailDTO
	var deadline sql.NullTime
	targets := append(saleListTargets(&sale.SaleListItemDTO, &deadline),
		&sale.ExternalStatus, &sale.ExternalStatusDetail, &sale.ClosedAt, &sale.ExternalUpdatedAt, &sale.LastSyncedAt, &sale.IncompleteReason,
		&sale.CustomerExternalID, &sale.CustomerNickname, &sale.CustomerEmail, &sale.CustomerPhone,
		&sale.TaxID, &sale.TaxName, &sale.TaxRegime, &sale.CFDIUse, &sale.BillingAddress,
		&sale.SubtotalAmount, &sale.DiscountAmount, &sale.ShippingAmount, &sale.TaxAmount, &sale.PaidAmount,
		&sale.MarketplaceFeeAmount, &sale.ShippingCostSeller,
	)
	if err := r.db.QueryRowContext(ctx, query, id).Scan(targets...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSaleNotFound
		}
		return nil, fmt.Errorf("error reading sale %d: %w", id, err)
	}
	if deadline.Valid {
		sale.DispatchDeadline = &deadline.Time
	}

	var err error
	if sale.Items, err = r.findItems(ctx, id); err != nil {
		return nil, err
	}
	if sale.Charges, err = r.findCharges(ctx, id); err != nil {
		return nil, err
	}
	if sale.Payments, err = r.findPayments(ctx, id); err != nil {
		return nil, err
	}
	if sale.Shipments, err = r.findShipments(ctx, id); err != nil {
		return nil, err
	}
	return &sale, nil
}

// FindPackSiblings lista las demás órdenes de la misma compra, por id.
func (r *SalesViewRepository) FindPackSiblings(ctx context.Context, connectionID int64, packID string, excludeID int64) ([]RelatedSaleDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, external_id, total_amount FROM ecom_sales
		WHERE connection_id = ? AND pack_id = ? AND id <> ? AND deleted_at IS NULL
		ORDER BY id ASC`,
		connectionID, packID, excludeID,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying pack sales: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, item *RelatedSaleDTO) error {
		return rows.Scan(&item.ID, &item.ExternalID, &item.TotalAmount)
	})
}

func (r *SalesViewRepository) findItems(ctx context.Context, saleID int64) ([]SaleItemDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.external_product_id, i.external_variation_id, i.sku, i.title, i.quantity,
		       i.unit_price, i.list_unit_price, i.discount_amount, i.tax_amount, i.total_amount, i.fee_amount,
		       i.product_id, p.name, i.match_method
		FROM ecom_sale_items i
		LEFT JOIN ecom_products p ON p.id = i.product_id AND p.deleted_at IS NULL
		WHERE i.sale_id = ? AND i.deleted_at IS NULL
		ORDER BY i.id ASC`, saleID)
	if err != nil {
		return nil, fmt.Errorf("error querying sale items: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, i *SaleItemDTO) error {
		return rows.Scan(&i.ID, &i.ExternalProductID, &i.ExternalVariationID, &i.SKU, &i.Title, &i.Quantity,
			&i.UnitPrice, &i.ListUnitPrice, &i.DiscountAmount, &i.TaxAmount, &i.TotalAmount, &i.FeeAmount,
			&i.ProductID, &i.ProductName, &i.MatchMethod)
	})
}

func (r *SalesViewRepository) findCharges(ctx context.Context, saleID int64) ([]SaleChargeDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT charge_type, paid_by, amount, description FROM ecom_sale_charges
		WHERE sale_id = ? AND deleted_at IS NULL ORDER BY charge_type ASC, id ASC`, saleID)
	if err != nil {
		return nil, fmt.Errorf("error querying sale charges: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, c *SaleChargeDTO) error {
		return rows.Scan(&c.Type, &c.PaidBy, &c.Amount, &c.Description)
	})
}

func (r *SalesViewRepository) findPayments(ctx context.Context, saleID int64) ([]SalePaymentDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id, method, payment_type, installments, external_status, amount, refunded_amount, approved_at
		FROM ecom_sale_payments WHERE sale_id = ? AND deleted_at IS NULL ORDER BY id ASC`, saleID)
	if err != nil {
		return nil, fmt.Errorf("error querying sale payments: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, p *SalePaymentDTO) error {
		return rows.Scan(&p.ExternalID, &p.Method, &p.PaymentType, &p.Installments, &p.ExternalStatus, &p.Amount, &p.RefundedAmount, &p.ApprovedAt)
	})
}

func (r *SalesViewRepository) findShipments(ctx context.Context, saleID int64) ([]SaleShipmentDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id, external_status, external_substatus, shipping_mode, logistic_type, carrier, tracking_number,
		       receiver_name, receiver_phone, receiver_address,
		       dispatch_deadline, estimated_delivery_at, shipped_at, delivered_at, buyer_cost, seller_cost
		FROM ecom_sale_shipments WHERE sale_id = ? AND deleted_at IS NULL ORDER BY id ASC`, saleID)
	if err != nil {
		return nil, fmt.Errorf("error querying sale shipments: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, s *SaleShipmentDTO) error {
		return rows.Scan(&s.ExternalID, &s.ExternalStatus, &s.ExternalSubstatus, &s.Mode, &s.LogisticType, &s.Carrier, &s.TrackingNumber,
			&s.ReceiverName, &s.ReceiverPhone, &s.ReceiverAddress,
			&s.DispatchDeadline, &s.EstimatedDeliveryAt, &s.ShippedAt, &s.DeliveredAt, &s.BuyerCost, &s.SellerCost)
	})
}

// scanRows recorre y cierra rows; siempre devuelve un slice no nil (JSON []).
func scanRows[T any](rows *sql.Rows, scan func(*sql.Rows, *T) error) ([]T, error) {
	defer rows.Close()
	result := make([]T, 0)
	for rows.Next() {
		var item T
		if err := scan(rows, &item); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	return result, nil
}

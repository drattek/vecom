package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Persistencia de las ventas importadas de los marketplaces (ADR 0010).

var ErrSaleNotFound = errors.New("sale not found")

// SaleRecord es una venta tal como la entrega el lector de un canal.
//
// Un puntero nil significa "no se obtuvo": al actualizar conserva el valor
// guardado. Lo mismo aplica a Items/Payments/Shipments: nil = no se pudo leer
// (no se tocan), slice vacío = la venta no tiene ninguno.
type SaleRecord struct {
	ConnectionID int64
	ExternalID   string
	OrderNumber  *string
	PackID       *string

	OrderStatus          string
	PaymentStatus        *string
	ShippingStatus       *string
	ExternalStatus       *string
	ExternalStatusDetail *string

	OrderedAt         time.Time
	ClosedAt          *time.Time
	ExternalUpdatedAt *time.Time

	CustomerExternalID *string
	CustomerName       *string
	CustomerNickname   *string
	CustomerEmail      *string
	CustomerPhone      *string

	TaxID          *string
	TaxName        *string
	TaxRegime      *string
	CFDIUse        *string
	BillingAddress *string

	CurrencyCode         *string
	SubtotalAmount       *float64
	DiscountAmount       *float64
	ShippingAmount       *float64
	TaxAmount            *float64
	TotalAmount          *float64
	PaidAmount           *float64
	MarketplaceFeeAmount *float64
	ShippingCostSeller   *float64

	// IncompleteReasons lista los datos secundarios que fallaron.
	IncompleteReasons []string

	RawPayload        *string
	BillingRawPayload *string

	Items     []SaleItemRecord
	Payments  []SalePaymentRecord
	Shipments []SaleShipmentRecord
	Charges   []SaleChargeRecord
	// ChargeSources son las consultas cuyos cargos vienen completos en
	// Charges: solo dentro de ellas se dan de baja los que ya no aparecen.
	ChargeSources []string
}

type SaleItemRecord struct {
	ExternalID          string
	ExternalProductID   *string
	ExternalVariationID *string
	// ListingExternalID es el id de la publicación tal como se guarda en
	// ecom_channel_product_map; no se persiste en la venta.
	ListingExternalID string
	SKU               *string
	Title             *string
	Quantity          float64
	UnitPrice         *float64
	ListUnitPrice     *float64
	DiscountAmount    *float64
	TaxAmount         *float64
	TotalAmount       *float64
	FeeAmount         *float64
	ProductID         *int64
	MatchMethod       *string
}

type SaleChargeRecord struct {
	ExternalID  string
	Source      string
	Type        string
	PaidBy      *string
	Amount      float64
	Description *string
}

type SalePaymentRecord struct {
	ExternalID     string
	Method         *string
	PaymentType    *string
	Installments   *int
	ExternalStatus *string
	Amount         *float64
	RefundedAmount *float64
	ApprovedAt     *time.Time
}

type SaleShipmentRecord struct {
	ExternalID          string
	ExternalStatus      *string
	ExternalSubstatus   *string
	Mode                *string
	LogisticType        *string
	Carrier             *string
	TrackingNumber      *string
	ReceiverName        *string
	ReceiverPhone       *string
	ReceiverAddress     *string
	DispatchDeadline    *time.Time
	EstimatedDeliveryAt *time.Time
	ShippedAt           *time.Time
	DeliveredAt         *time.Time
	BuyerCost           *float64
	SellerCost          *float64
	RawPayload          *string
}

// SaleKnownState es lo que un lector necesita saber de una venta ya guardada
// para no volver a pedir datos al marketplace.
type SaleKnownState struct {
	ExternalUpdatedAt *time.Time
	Incomplete        bool
	HasBilling        bool
}

type SalesRepository struct {
	db Querier
}

func NewSalesRepository(db Querier) *SalesRepository {
	return &SalesRepository{db: db}
}

// FindKnownState devuelve nil, nil si la venta todavía no está guardada.
func (r *SalesRepository) FindKnownState(ctx context.Context, connectionID int64, externalID string) (*SaleKnownState, error) {
	var updatedAt sql.NullTime
	var state SaleKnownState
	err := r.db.QueryRowContext(ctx, `
		SELECT external_updated_at, is_incomplete, (tax_id IS NOT NULL OR tax_name IS NOT NULL)
		FROM ecom_sales WHERE connection_id = ? AND external_id = ?`,
		connectionID, externalID,
	).Scan(&updatedAt, &state.Incomplete, &state.HasBilling)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading known sale state: %w", err)
	}
	if updatedAt.Valid {
		state.ExternalUpdatedAt = &updatedAt.Time
	}
	return &state, nil
}

// FindOpenExternalIDs lista las ventas que pueden cambiar sin que cambie la
// fecha de la orden: pago pendiente, envío sin cerrar o datos incompletos.
// orderedSince acota la antigüedad para que el costo no crezca sin límite.
func (r *SalesRepository) FindOpenExternalIDs(ctx context.Context, connectionID int64, orderedSince time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id FROM ecom_sales
		WHERE connection_id = ? AND deleted_at IS NULL AND ordered_at >= ?
		  AND (
		        is_incomplete = 1
		        OR (order_status <> 'cancelled' AND (
		              payment_status = 'pending'
		              OR shipping_status NOT IN ('delivered', 'cancelled', 'not_applicable')
		        ))
		  )
		ORDER BY ordered_at ASC`,
		connectionID, orderedSince,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying open sales: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("error scanning open sale: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating open sales: %w", err)
	}
	return ids, nil
}

// FindProductIDByListing resuelve el producto local de una publicación. Solo
// responde si la publicación apunta a un único producto.
func (r *SalesRepository) FindProductIDByListing(ctx context.Context, connectionID int64, listingExternalID string) (*int64, error) {
	var count int
	var productID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT product_id), MIN(product_id)
		FROM ecom_channel_product_map
		WHERE connection_id = ? AND external_id = ? AND deleted_at IS NULL`,
		connectionID, listingExternalID,
	).Scan(&count, &productID)
	if err != nil {
		return nil, fmt.Errorf("error resolving product by listing: %w", err)
	}
	if count != 1 || !productID.Valid {
		return nil, nil
	}
	return &productID.Int64, nil
}

func (r *SalesRepository) FindProductIDBySKU(ctx context.Context, sku string) (*int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM ecom_products WHERE sku = ? AND deleted_at IS NULL LIMIT 1`, sku,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error resolving product by sku: %w", err)
	}
	return &id, nil
}

// Save inserta o actualiza la venta y sus hijos. Ejecuta varias sentencias:
// llamar sobre un repositorio construido con la transacción (mysql.WithinTx).
// actorID es nil cuando escribe el job.
func (r *SalesRepository) Save(ctx context.Context, record SaleRecord, actorID *int64, now time.Time) (int64, error) {
	var saleID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM ecom_sales WHERE connection_id = ? AND external_id = ? FOR UPDATE`,
		record.ConnectionID, record.ExternalID,
	).Scan(&saleID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		saleID, err = r.insertSale(ctx, record, actorID, now)
	case err == nil:
		err = r.updateSale(ctx, saleID, record, actorID, now)
	default:
		return 0, fmt.Errorf("error locking sale: %w", err)
	}
	if err != nil {
		return 0, err
	}

	if err := r.saveItems(ctx, saleID, record.Items, actorID, now); err != nil {
		return 0, err
	}
	if err := r.savePayments(ctx, saleID, record.Payments, actorID, now); err != nil {
		return 0, err
	}
	if err := r.saveShipments(ctx, saleID, record.Shipments, actorID, now); err != nil {
		return 0, err
	}
	if err := r.saveCharges(ctx, saleID, record.Charges, record.ChargeSources, actorID, now); err != nil {
		return 0, err
	}
	return saleID, nil
}

func incompleteColumns(reasons []string) (bool, *string) {
	if len(reasons) == 0 {
		return false, nil
	}
	joined := truncate(strings.Join(reasons, "; "), 500)
	return true, &joined
}

// saleOptionalColumns son las columnas que un valor ausente no sobrescribe.
// El orden coincide con saleOptionalValues.
var saleOptionalColumns = []string{
	"order_number", "pack_id", "external_status", "external_status_detail",
	"closed_at", "external_updated_at",
	"customer_external_id", "customer_name", "customer_nickname", "customer_email", "customer_phone",
	"tax_id", "tax_name", "tax_regime", "cfdi_use", "billing_address",
	"currency_code", "subtotal_amount", "discount_amount", "shipping_amount", "tax_amount",
	"total_amount", "paid_amount", "marketplace_fee_amount", "shipping_cost_seller",
	"raw_payload", "billing_raw_payload",
}

func saleOptionalValues(s SaleRecord) []any {
	return []any{
		clip(s.OrderNumber, 64), clip(s.PackID, 64), clip(s.ExternalStatus, 64), clip(s.ExternalStatusDetail, 255),
		s.ClosedAt, s.ExternalUpdatedAt,
		clip(s.CustomerExternalID, 64), clip(s.CustomerName, 255), clip(s.CustomerNickname, 100), clip(s.CustomerEmail, 255), clip(s.CustomerPhone, 64),
		clip(s.TaxID, 32), clip(s.TaxName, 255), clip(s.TaxRegime, 120), clip(s.CFDIUse, 120), clip(s.BillingAddress, 600),
		clip(s.CurrencyCode, 8), s.SubtotalAmount, s.DiscountAmount, s.ShippingAmount, s.TaxAmount,
		s.TotalAmount, s.PaidAmount, s.MarketplaceFeeAmount, s.ShippingCostSeller,
		s.RawPayload, s.BillingRawPayload,
	}
}

func (r *SalesRepository) insertSale(ctx context.Context, s SaleRecord, actorID *int64, now time.Time) (int64, error) {
	incomplete, reason := incompleteColumns(s.IncompleteReasons)
	columns := []string{
		"connection_id", "external_id", "order_status", "payment_status", "shipping_status",
		"ordered_at", "is_incomplete", "incomplete_reason", "last_synced_at", "created_by",
	}
	args := []any{
		s.ConnectionID, s.ExternalID, s.OrderStatus,
		valueOr(s.PaymentStatus, "unknown"), valueOr(s.ShippingStatus, "unknown"),
		s.OrderedAt, incomplete, reason, now, actorID,
	}
	columns = append(columns, saleOptionalColumns...)
	args = append(args, saleOptionalValues(s)...)

	query := fmt.Sprintf("INSERT INTO ecom_sales (%s) VALUES (%s)",
		strings.Join(columns, ", "), placeholders(len(columns)))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("error inserting sale: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error reading sale id: %w", err)
	}
	return id, nil
}

func (r *SalesRepository) updateSale(ctx context.Context, saleID int64, s SaleRecord, actorID *int64, now time.Time) error {
	incomplete, reason := incompleteColumns(s.IncompleteReasons)
	sets := []string{
		"order_status = ?",
		"payment_status = COALESCE(?, payment_status)",
		"shipping_status = COALESCE(?, shipping_status)",
		"ordered_at = ?",
		"is_incomplete = ?",
		"incomplete_reason = ?",
		"last_synced_at = ?",
		"updated_by = ?",
		"deleted_at = NULL",
	}
	args := []any{s.OrderStatus, s.PaymentStatus, s.ShippingStatus, s.OrderedAt, incomplete, reason, now, actorID}
	for _, column := range saleOptionalColumns {
		sets = append(sets, fmt.Sprintf("%s = COALESCE(?, %s)", column, column))
	}
	args = append(args, saleOptionalValues(s)...)
	args = append(args, saleID)

	query := fmt.Sprintf("UPDATE ecom_sales SET %s WHERE id = ?", strings.Join(sets, ", "))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("error updating sale %d: %w", saleID, err)
	}
	return nil
}

// upsertChild inserta o actualiza una fila hija por su clave (sale_id,
// external_id). Las columnas "required" siempre se sobrescriben; las
// "optional" conservan el valor guardado si llega NULL.
func (r *SalesRepository) upsertChild(
	ctx context.Context, table string, saleID int64, externalID string,
	required []string, requiredValues []any,
	optional []string, optionalValues []any,
	actorID *int64,
) error {
	columns := append([]string{"sale_id", "external_id", "created_by"}, required...)
	columns = append(columns, optional...)
	args := append([]any{saleID, externalID, actorID}, requiredValues...)
	args = append(args, optionalValues...)

	updates := []string{"deleted_at = NULL", "updated_by = ?"}
	for _, column := range required {
		updates = append(updates, fmt.Sprintf("%s = VALUES(%s)", column, column))
	}
	for _, column := range optional {
		updates = append(updates, fmt.Sprintf("%s = COALESCE(VALUES(%s), %s)", column, column, column))
	}
	args = append(args, actorID)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		table, strings.Join(columns, ", "), placeholders(len(columns)), strings.Join(updates, ", "))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("error saving row in %s: %w", table, err)
	}
	return nil
}

// softDeleteMissing da de baja los hijos que ya no vienen en la respuesta.
func (r *SalesRepository) softDeleteMissing(
	ctx context.Context, table string, saleID int64, keep []string,
	extraCondition string, extraArgs []any, actorID *int64, now time.Time,
) error {
	query := fmt.Sprintf("UPDATE %s SET deleted_at = ?, updated_by = ? WHERE sale_id = ? AND deleted_at IS NULL%s", table, extraCondition)
	args := append([]any{now, actorID, saleID}, extraArgs...)
	if len(keep) > 0 {
		query += fmt.Sprintf(" AND external_id NOT IN (%s)", placeholders(len(keep)))
		for _, id := range keep {
			args = append(args, id)
		}
	}
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("error removing stale rows from %s: %w", table, err)
	}
	return nil
}

func (r *SalesRepository) saveItems(ctx context.Context, saleID int64, items []SaleItemRecord, actorID *int64, now time.Time) error {
	if items == nil {
		return nil
	}
	keep := make([]string, 0, len(items))
	for _, item := range items {
		externalID := truncate(item.ExternalID, 120)
		keep = append(keep, externalID)
		if err := r.upsertChild(ctx, "ecom_sale_items", saleID, externalID,
			[]string{"quantity"}, []any{item.Quantity},
			[]string{
				"external_product_id", "external_variation_id", "sku", "title",
				"unit_price", "list_unit_price", "discount_amount", "tax_amount", "total_amount", "fee_amount",
				"product_id", "match_method",
			},
			[]any{
				clip(item.ExternalProductID, 64), clip(item.ExternalVariationID, 64), clip(item.SKU, 255), clip(item.Title, 500),
				item.UnitPrice, item.ListUnitPrice, item.DiscountAmount, item.TaxAmount, item.TotalAmount, item.FeeAmount,
				item.ProductID, item.MatchMethod,
			},
			actorID,
		); err != nil {
			return err
		}
	}
	return r.softDeleteMissing(ctx, "ecom_sale_items", saleID, keep, "", nil, actorID, now)
}

func (r *SalesRepository) savePayments(ctx context.Context, saleID int64, payments []SalePaymentRecord, actorID *int64, now time.Time) error {
	if payments == nil {
		return nil
	}
	keep := make([]string, 0, len(payments))
	for _, payment := range payments {
		externalID := truncate(payment.ExternalID, 64)
		keep = append(keep, externalID)
		if err := r.upsertChild(ctx, "ecom_sale_payments", saleID, externalID,
			nil, nil,
			[]string{"method", "payment_type", "installments", "external_status", "amount", "refunded_amount", "approved_at"},
			[]any{
				clip(payment.Method, 120), clip(payment.PaymentType, 64), payment.Installments,
				clip(payment.ExternalStatus, 64), payment.Amount, payment.RefundedAmount, payment.ApprovedAt,
			},
			actorID,
		); err != nil {
			return err
		}
	}
	return r.softDeleteMissing(ctx, "ecom_sale_payments", saleID, keep, "", nil, actorID, now)
}

func (r *SalesRepository) saveShipments(ctx context.Context, saleID int64, shipments []SaleShipmentRecord, actorID *int64, now time.Time) error {
	if shipments == nil {
		return nil
	}
	keep := make([]string, 0, len(shipments))
	for _, s := range shipments {
		externalID := truncate(s.ExternalID, 64)
		keep = append(keep, externalID)
		if err := r.upsertChild(ctx, "ecom_sale_shipments", saleID, externalID,
			nil, nil,
			[]string{
				"external_status", "external_substatus", "shipping_mode", "logistic_type", "carrier", "tracking_number",
				"receiver_name", "receiver_phone", "receiver_address",
				"dispatch_deadline", "estimated_delivery_at", "shipped_at", "delivered_at",
				"buyer_cost", "seller_cost", "raw_payload",
			},
			[]any{
				clip(s.ExternalStatus, 64), clip(s.ExternalSubstatus, 64), clip(s.Mode, 32), clip(s.LogisticType, 64), clip(s.Carrier, 160), clip(s.TrackingNumber, 120),
				clip(s.ReceiverName, 255), clip(s.ReceiverPhone, 64), clip(s.ReceiverAddress, 600),
				s.DispatchDeadline, s.EstimatedDeliveryAt, s.ShippedAt, s.DeliveredAt,
				s.BuyerCost, s.SellerCost, s.RawPayload,
			},
			actorID,
		); err != nil {
			return err
		}
	}
	return r.softDeleteMissing(ctx, "ecom_sale_shipments", saleID, keep, "", nil, actorID, now)
}

func (r *SalesRepository) saveCharges(ctx context.Context, saleID int64, charges []SaleChargeRecord, sources []string, actorID *int64, now time.Time) error {
	keep := make([]string, 0, len(charges))
	for _, charge := range charges {
		externalID := truncate(charge.ExternalID, 160)
		keep = append(keep, externalID)
		if err := r.upsertChild(ctx, "ecom_sale_charges", saleID, externalID,
			[]string{"source", "charge_type", "amount"}, []any{charge.Source, charge.Type, charge.Amount},
			[]string{"paid_by", "description"}, []any{charge.PaidBy, clip(charge.Description, 255)},
			actorID,
		); err != nil {
			return err
		}
	}
	for _, source := range sources {
		if err := r.softDeleteMissing(ctx, "ecom_sale_charges", saleID, keep, " AND source = ?", []any{source}, actorID, now); err != nil {
			return err
		}
	}
	return nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// truncate recorta por caracteres (no bytes) al ancho de la columna: un valor
// largo del marketplace no debe impedir guardar la venta.
func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func clip(value *string, max int) *string {
	if value == nil {
		return nil
	}
	clipped := truncate(*value, max)
	return &clipped
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

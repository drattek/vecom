package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrMeliOrderNotFound = errors.New("meli order not found")

// MeliOrderDTO maps a row of ecom_meli_orders. Only the typed columns are
// decoded; the full raw payload stays in RawPayload for fase 2 consumers
// (Odoo sale.order creation, CFDI, etc).
type MeliOrderDTO struct {
	ID                      int64
	ConnectionID            int64
	MeliOrderID             string
	Status                  string
	StatusDetail            *string
	DateCreated             *time.Time
	DateClosed              *time.Time
	LastUpdated             *time.Time
	TotalAmount             *float64
	PaidAmount              *float64
	CurrencyID              *string
	BuyerMeliUserID         *int64
	BuyerNickname           *string
	BuyerFirstName          *string
	BuyerLastName           *string
	BuyerEmail              *string
	BuyerDocType            *string
	BuyerDocNumber          *string
	ShippingID              *string
	PackID                  *string
	Tags                    *string
	RawPayload              string
	FirstSeenNotificationID *int64
	LastSeenNotificationID  *int64
}

// UpsertMeliOrderInput is what the processor passes to the repository for
// a single order payload. Every pointer field is nullable — MELI returns
// subsets depending on order state.
type UpsertMeliOrderInput struct {
	ConnectionID            int64
	MeliOrderID             string
	Status                  string
	StatusDetail            *string
	DateCreated             *time.Time
	DateClosed              *time.Time
	LastUpdated             *time.Time
	TotalAmount             *float64
	PaidAmount              *float64
	CurrencyID              *string
	BuyerMeliUserID         *int64
	BuyerNickname           *string
	BuyerFirstName          *string
	BuyerLastName           *string
	BuyerEmail              *string
	BuyerDocType            *string
	BuyerDocNumber          *string
	ShippingID              *string
	PackID                  *string
	Tags                    *string
	RawPayload              string
	SeenNotificationID      *int64
	Items                   []UpsertMeliOrderItemInput
}

type UpsertMeliOrderItemInput struct {
	ItemMeliID      string
	ItemVariationID *string
	SellerSKU       *string
	Title           *string
	CategoryID      *string
	UnitPrice       *float64
	FullUnitPrice   *float64
	Quantity        *int
	CurrencyID      *string
	SaleFee         *float64
	ListingTypeID   *string
	RawPayload      string
}

type MeliOrderRepository struct {
	db *sql.DB
}

func NewMeliOrderRepository(db *sql.DB) *MeliOrderRepository {
	return &MeliOrderRepository{db: db}
}

// Upsert inserts the order on first sight or updates typed columns and the
// raw payload on subsequent sightings (MELI redelivers the same order id
// as the shipment/payment state changes). Items are replaced in bulk on
// every call — simpler than diffing and MELI's `order_items` is the full
// list, never a delta.
func (r *MeliOrderRepository) Upsert(ctx context.Context, input UpsertMeliOrderInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("error beginning tx for meli order upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// first_seen keeps whatever was there; last_seen always wins.
	upsertQuery := `
		INSERT INTO ecom_meli_orders
		  (connection_id, meli_order_id, status, status_detail,
		   date_created, date_closed, last_updated,
		   total_amount, paid_amount, currency_id,
		   buyer_meli_user_id, buyer_nickname, buyer_first_name, buyer_last_name,
		   buyer_email, buyer_doc_type, buyer_doc_number,
		   shipping_id, pack_id, tags, raw_payload,
		   first_seen_notification_id, last_seen_notification_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  status = VALUES(status),
		  status_detail = VALUES(status_detail),
		  date_created = COALESCE(VALUES(date_created), date_created),
		  date_closed = VALUES(date_closed),
		  last_updated = VALUES(last_updated),
		  total_amount = VALUES(total_amount),
		  paid_amount = VALUES(paid_amount),
		  currency_id = VALUES(currency_id),
		  buyer_meli_user_id = COALESCE(VALUES(buyer_meli_user_id), buyer_meli_user_id),
		  buyer_nickname = COALESCE(VALUES(buyer_nickname), buyer_nickname),
		  buyer_first_name = COALESCE(VALUES(buyer_first_name), buyer_first_name),
		  buyer_last_name = COALESCE(VALUES(buyer_last_name), buyer_last_name),
		  buyer_email = COALESCE(VALUES(buyer_email), buyer_email),
		  buyer_doc_type = COALESCE(VALUES(buyer_doc_type), buyer_doc_type),
		  buyer_doc_number = COALESCE(VALUES(buyer_doc_number), buyer_doc_number),
		  shipping_id = VALUES(shipping_id),
		  pack_id = VALUES(pack_id),
		  tags = VALUES(tags),
		  raw_payload = VALUES(raw_payload),
		  last_seen_notification_id = VALUES(last_seen_notification_id)
	`

	_, err = tx.ExecContext(ctx, upsertQuery,
		input.ConnectionID, input.MeliOrderID, input.Status, input.StatusDetail,
		input.DateCreated, input.DateClosed, input.LastUpdated,
		input.TotalAmount, input.PaidAmount, input.CurrencyID,
		input.BuyerMeliUserID, input.BuyerNickname, input.BuyerFirstName, input.BuyerLastName,
		input.BuyerEmail, input.BuyerDocType, input.BuyerDocNumber,
		input.ShippingID, input.PackID, input.Tags, input.RawPayload,
		input.SeenNotificationID, input.SeenNotificationID,
	)
	if err != nil {
		return 0, fmt.Errorf("error upserting meli order: %w", err)
	}

	var orderPK int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM ecom_meli_orders WHERE connection_id = ? AND meli_order_id = ? LIMIT 1`,
		input.ConnectionID, input.MeliOrderID,
	).Scan(&orderPK); err != nil {
		return 0, fmt.Errorf("error reading meli order pk after upsert: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM ecom_meli_order_items WHERE meli_order_pk = ?`, orderPK,
	); err != nil {
		return 0, fmt.Errorf("error wiping prior meli order items: %w", err)
	}

	if len(input.Items) > 0 {
		insertItem := `
			INSERT INTO ecom_meli_order_items
			  (meli_order_pk, item_meli_id, item_variation_id, seller_sku, title, category_id,
			   unit_price, full_unit_price, quantity, currency_id, sale_fee, listing_type_id, raw_payload)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		for _, item := range input.Items {
			if _, err := tx.ExecContext(ctx, insertItem,
				orderPK, item.ItemMeliID, item.ItemVariationID, item.SellerSKU, item.Title, item.CategoryID,
				item.UnitPrice, item.FullUnitPrice, item.Quantity, item.CurrencyID, item.SaleFee, item.ListingTypeID,
				item.RawPayload,
			); err != nil {
				return 0, fmt.Errorf("error inserting meli order item (sku=%v): %w", derefMeliOrderString(item.SellerSKU), err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing meli order upsert tx: %w", err)
	}

	return orderPK, nil
}

func derefMeliOrderString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// UpsertShipmentInput mirrors the typed columns of ecom_meli_shipments.
type UpsertShipmentInput struct {
	ConnectionID    int64
	MeliShipmentID  string
	MeliOrderID     *string
	Status          *string
	Substatus       *string
	Mode            *string
	TrackingNumber  *string
	TrackingMethod  *string
	ServiceID       *string
	DateCreated     *time.Time
	LastUpdated     *time.Time
	RawPayload      string
}

// UpsertShipment lives in the same repository as orders for proximity and
// because the processor usually calls them back-to-back; it keeps its own
// table under the shadow design.
func (r *MeliOrderRepository) UpsertShipment(ctx context.Context, input UpsertShipmentInput) error {
	query := `
		INSERT INTO ecom_meli_shipments
		  (connection_id, meli_shipment_id, meli_order_id, status, substatus, mode,
		   tracking_number, tracking_method, service_id, date_created, last_updated, raw_payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  meli_order_id = COALESCE(VALUES(meli_order_id), meli_order_id),
		  status = VALUES(status),
		  substatus = VALUES(substatus),
		  mode = VALUES(mode),
		  tracking_number = VALUES(tracking_number),
		  tracking_method = VALUES(tracking_method),
		  service_id = VALUES(service_id),
		  date_created = COALESCE(VALUES(date_created), date_created),
		  last_updated = VALUES(last_updated),
		  raw_payload = VALUES(raw_payload)
	`

	if _, err := r.db.ExecContext(ctx, query,
		input.ConnectionID, input.MeliShipmentID, input.MeliOrderID, input.Status, input.Substatus, input.Mode,
		input.TrackingNumber, input.TrackingMethod, input.ServiceID, input.DateCreated, input.LastUpdated,
		input.RawPayload,
	); err != nil {
		return fmt.Errorf("error upserting meli shipment: %w", err)
	}

	return nil
}

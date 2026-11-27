package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Valores de ecom_channel_listing_closures.
const (
	// ClosureDefinitive: la cerró MercadoLibre y no se recupera. Es la única
	// que se avisa.
	ClosureDefinitive = "definitive"
	// ClosureFixable: MercadoLibre permite corregirla.
	ClosureFixable = "fixable"
	// ClosureSystem: no se atribuye a MercadoLibre (la cerró este sistema o el
	// vendedor).
	ClosureSystem = "system"

	ClosureReasonPending     = "pending"
	ClosureReasonFound       = "found"
	ClosureReasonUnavailable = "unavailable"
)

// ListingClosureDTO es una publicación cerrada y su motivo.
type ListingClosureDTO struct {
	ID             int64
	ExternalID     string
	Kind           string
	ReasonStatus   string
	SubStatus      string
	ModerationName string
	Reason         string
	Remedy         string
	DetectedAt     time.Time
	// Solo en ListPendingNotice.
	SKU            string
	ProductName    string
	ConnectionName string
}

type SaveListingClosureInput struct {
	ConnectionID   int64
	ProductID      int64
	MapID          int64
	ExternalID     string
	ExternalStatus string
	SubStatus      string
	Kind           string
	ReasonStatus   string
	ModerationName string
	Reason         string
	Remedy         string
	DetectedAt     time.Time
	NotifiedAt     *time.Time
}

type ListingClosureRepository struct {
	db Querier
}

func NewListingClosureRepository(db Querier) *ListingClosureRepository {
	return &ListingClosureRepository{db: db}
}

// ListActive devuelve los cierres vigentes (no reabiertos) de una conexión.
func (r *ListingClosureRepository) ListActive(ctx context.Context, connectionID int64) ([]ListingClosureDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, external_id, closure_kind, reason_status, COALESCE(external_sub_status, ''), detected_at
		FROM ecom_channel_listing_closures
		WHERE connection_id = ? AND reopened_at IS NULL AND deleted_at IS NULL
	`, connectionID)
	if err != nil {
		return nil, fmt.Errorf("listing closures of connection %d: %w", connectionID, err)
	}
	defer rows.Close()

	closures := make([]ListingClosureDTO, 0)
	for rows.Next() {
		var closure ListingClosureDTO
		if err := rows.Scan(&closure.ID, &closure.ExternalID, &closure.Kind, &closure.ReasonStatus,
			&closure.SubStatus, &closure.DetectedAt); err != nil {
			return nil, fmt.Errorf("reading closure: %w", err)
		}
		closures = append(closures, closure)
	}
	return closures, rows.Err()
}

// Save no hace nada si la publicación ya tiene un cierre vigente.
func (r *ListingClosureRepository) Save(ctx context.Context, in SaveListingClosureInput) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_channel_listing_closures
			(connection_id, product_id, channel_product_map_id, external_id, external_status, external_sub_status,
			 closure_kind, reason_status, moderation_name, reason, remedy, detected_at, notified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE external_id = external_id
	`, in.ConnectionID, in.ProductID, in.MapID, in.ExternalID,
		nullableString(truncateRunes(in.ExternalStatus, 32)), nullableString(truncateRunes(in.SubStatus, 255)),
		in.Kind, in.ReasonStatus, nullableString(truncateRunes(in.ModerationName, 120)),
		nullableString(in.Reason), nullableString(in.Remedy), in.DetectedAt, in.NotifiedAt)
	if err != nil {
		return fmt.Errorf("saving closure of %s: %w", in.ExternalID, err)
	}
	return nil
}

func (r *ListingClosureRepository) SetResolution(ctx context.Context, id int64, kind, reasonStatus, moderationName, reason, remedy string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_listing_closures
		SET closure_kind = ?, reason_status = ?, moderation_name = ?, reason = ?, remedy = ?
		WHERE id = ?
	`, kind, reasonStatus, nullableString(truncateRunes(moderationName, 120)),
		nullableString(reason), nullableString(remedy), id)
	if err != nil {
		return fmt.Errorf("updating closure %d: %w", id, err)
	}
	return nil
}

func (r *ListingClosureRepository) MarkReopened(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_listing_closures SET reopened_at = ? WHERE id = ?
	`, at, id)
	if err != nil {
		return fmt.Errorf("reopening closure %d: %w", id, err)
	}
	return nil
}

// ListPendingNotice: cierres definitivos sin avisar, con el motivo ya
// consultado y detectados desde since.
func (r *ListingClosureRepository) ListPendingNotice(ctx context.Context, since time.Time) ([]ListingClosureDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.external_id, c.closure_kind, c.reason_status, COALESCE(c.external_sub_status, ''),
		       COALESCE(c.moderation_name, ''), COALESCE(c.reason, ''), COALESCE(c.remedy, ''), c.detected_at,
		       p.sku, p.name, cc.name
		FROM ecom_channel_listing_closures c
		JOIN ecom_products p ON p.id = c.product_id
		JOIN ecom_channel_connections cc ON cc.id = c.connection_id
		WHERE c.closure_kind = 'definitive' AND c.notified_at IS NULL AND c.reason_status <> 'pending'
		  AND c.reopened_at IS NULL AND c.deleted_at IS NULL AND c.detected_at >= ?
		ORDER BY p.sku, c.id
	`, since)
	if err != nil {
		return nil, fmt.Errorf("listing closures to notify: %w", err)
	}
	defer rows.Close()

	closures := make([]ListingClosureDTO, 0)
	for rows.Next() {
		var c ListingClosureDTO
		if err := rows.Scan(&c.ID, &c.ExternalID, &c.Kind, &c.ReasonStatus, &c.SubStatus,
			&c.ModerationName, &c.Reason, &c.Remedy, &c.DetectedAt,
			&c.SKU, &c.ProductName, &c.ConnectionName); err != nil {
			return nil, fmt.Errorf("reading closure to notify: %w", err)
		}
		closures = append(closures, c)
	}
	return closures, rows.Err()
}

func (r *ListingClosureRepository) MarkNotified(ctx context.Context, ids []int64, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, at)
	for _, id := range ids {
		args = append(args, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	_, err := r.db.ExecContext(ctx,
		"UPDATE ecom_channel_listing_closures SET notified_at = ? WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return fmt.Errorf("marking closures as notified: %w", err)
	}
	return nil
}

// HasStatusEvent indica si la bitácora tiene un cambio de estado de esa fila con
// ese mensaje (sirve para reconocer los cierres que hizo este sistema).
func (r *ListingClosureRepository) HasStatusEvent(ctx context.Context, mapID int64, message string) (bool, error) {
	var found int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM ecom_channel_listing_events
		WHERE channel_product_map_id = ? AND action = 'status_sync' AND message = ?
		LIMIT 1
	`, mapID, message).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading status events of listing %d: %w", mapID, err)
	}
	return true, nil
}

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Channel listing event actions/outcomes — mirror the ENUMs on
// ecom_channel_listing_events (see infrastructure/mysql/
// 2026-10-14_channel_listing_events.sql).
const (
	ListingEventPublish    = "publish"
	ListingEventRefresh    = "refresh"
	ListingEventResync     = "resync"
	ListingEventPause      = "pause"
	ListingEventReactivate = "reactivate"
	ListingEventStatusSync = "status_sync"

	ListingEventSuccess = "success"
	ListingEventError   = "error"
)

// Listing view states — what the "Publicaciones" screen shows, derived from
// ecom_channel_product_map.status/paused_reason plus the event log for
// products that never got a listing.
const (
	ListingStatePublished       = "published"
	ListingStatePausedZeroStock = "paused_zero_stock"
	ListingStatePausedManual    = "paused_manual"
	ListingStateUnderReview     = "under_review"
	ListingStateClosed          = "closed"
	ListingStatePending         = "pending"
	ListingStateError           = "error"
	ListingStateNotPublished    = "not_published"
)

type RecordListingEventInput struct {
	ProductID           int64
	ConnectionID        int64
	VehicleFitmentID    *int64
	ChannelProductMapID *int64
	ExternalID          string
	Action              string
	Outcome             string
	StatusBefore        string
	StatusAfter         string
	Message             string
}

type ListingEventDTO struct {
	ID                  int64     `json:"id"`
	ProductID           int64     `json:"productId"`
	ConnectionID        int64     `json:"connectionId"`
	VehicleFitmentID    *int64    `json:"vehicleFitmentId,omitempty"`
	ChannelProductMapID *int64    `json:"channelProductMapId,omitempty"`
	ExternalID          *string   `json:"externalId,omitempty"`
	Action              string    `json:"action"`
	Outcome             string    `json:"outcome"`
	StatusBefore        *string   `json:"statusBefore,omitempty"`
	StatusAfter         *string   `json:"statusAfter,omitempty"`
	Message             *string   `json:"message,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

// ListingViewFilter narrows the combined listings view. ChannelCode is
// required; every other field is optional (zero value = no filter).
type ListingViewFilter struct {
	ChannelCode  string
	ConnectionID int64
	State        string
	WithError    bool
	Search       string
	Offset       int
	PageSize     int
}

// ListingViewRow is one line of the "Publicaciones" screen: either a real
// ecom_channel_product_map row (Kind "listing") or a product whose last
// publish attempt failed and has no live listing (Kind "not_published").
type ListingViewRow struct {
	Kind               string     `json:"kind"`
	ID                 int64      `json:"id"`
	ProductID          int64      `json:"productId"`
	SKU                string     `json:"sku"`
	ProductName        string     `json:"productName"`
	ConnectionID       int64      `json:"connectionId"`
	ConnectionName     string     `json:"connectionName"`
	VehicleFitmentID   *int64     `json:"vehicleFitmentId,omitempty"`
	ListingTitle       *string    `json:"listingTitle,omitempty"`
	ExternalID         *string    `json:"externalId,omitempty"`
	ExternalCategoryID *string    `json:"externalCategoryId,omitempty"`
	RawStatus          *string    `json:"rawStatus,omitempty"`
	PausedReason       *string    `json:"pausedReason,omitempty"`
	State              string     `json:"state"`
	LastError          *string    `json:"lastError,omitempty"`
	LastActivityAt     *time.Time `json:"lastActivityAt,omitempty"`
	Attempts           int64      `json:"attempts"`
}

type PaginatedListingView struct {
	Total        int64            `json:"total"`
	Offset       int              `json:"offset"`
	PageSize     int              `json:"pageSize"`
	Listings     []ListingViewRow `json:"listings"`
	StateCounts  map[string]int64 `json:"stateCounts"`
	WithErrorCnt int64            `json:"withErrorCount"`
}

type ChannelListingEventsRepository struct {
	db Querier
}

func NewChannelListingEventsRepository(db Querier) *ChannelListingEventsRepository {
	return &ChannelListingEventsRepository{db: db}
}

func (r *ChannelListingEventsRepository) Record(ctx context.Context, in RecordListingEventInput) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_channel_listing_events
			(product_id, connection_id, vehicle_fitment_id, channel_product_map_id, external_id,
			 action, outcome, status_before, status_after, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		in.ProductID, in.ConnectionID, in.VehicleFitmentID, in.ChannelProductMapID,
		nullableString(in.ExternalID), in.Action, in.Outcome,
		nullableString(in.StatusBefore), nullableString(in.StatusAfter), nullableString(in.Message),
	)
	if err != nil {
		return fmt.Errorf("error recording listing event: %w", err)
	}
	return nil
}

// FindByListing returns the full history of (product, connection, fitment),
// newest first.
func (r *ChannelListingEventsRepository) FindByListing(ctx context.Context, productID, connectionID int64, vehicleFitmentID *int64, limit int) ([]ListingEventDTO, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var fitmentKey int64
	if vehicleFitmentID != nil {
		fitmentKey = *vehicleFitmentID
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, product_id, connection_id, vehicle_fitment_id, channel_product_map_id, external_id,
		       action, outcome, status_before, status_after, message, created_at
		FROM ecom_channel_listing_events
		WHERE product_id = ? AND connection_id = ? AND vehicle_fitment_key = ?
		ORDER BY id DESC
		LIMIT ?
	`, productID, connectionID, fitmentKey, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying listing events: %w", err)
	}
	defer rows.Close()

	out := make([]ListingEventDTO, 0)
	for rows.Next() {
		var e ListingEventDTO
		var fitment, mapID sql.NullInt64
		var externalID, before, after, message sql.NullString
		if err := rows.Scan(&e.ID, &e.ProductID, &e.ConnectionID, &fitment, &mapID, &externalID,
			&e.Action, &e.Outcome, &before, &after, &message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning listing event: %w", err)
		}
		if fitment.Valid {
			e.VehicleFitmentID = &fitment.Int64
		}
		if mapID.Valid {
			e.ChannelProductMapID = &mapID.Int64
		}
		if externalID.Valid {
			e.ExternalID = &externalID.String
		}
		if before.Valid {
			e.StatusBefore = &before.String
		}
		if after.Valid {
			e.StatusAfter = &after.String
		}
		if message.Valid {
			e.Message = &message.String
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listing events: %w", err)
	}
	return out, nil
}

// listingViewSource is the UNION of live listings and never-published
// products whose latest publish attempt failed. Takes the channel code twice.
const listingViewSource = `
	SELECT 'listing' AS kind, cpm.id AS id, cpm.product_id, p.sku, p.name AS product_name,
	       cpm.connection_id, cc.name AS connection_name, cpm.vehicle_fitment_id,
	       cpm.listing_title, cpm.external_id, cpm.external_category_id,
	       cpm.status AS raw_status, cpm.paused_reason,
	       CASE
	         WHEN cpm.status = 'synced' THEN 'published'
	         WHEN cpm.status = 'paused' AND cpm.paused_reason = 'zero_stock' THEN 'paused_zero_stock'
	         WHEN cpm.status = 'paused' THEN 'paused_manual'
	         WHEN cpm.status = 'under_review' THEN 'under_review'
	         WHEN cpm.status = 'closed' THEN 'closed'
	         WHEN cpm.status = 'error' THEN 'error'
	         ELSE 'pending'
	       END AS state,
	       cpm.last_error,
	       COALESCE(cpm.last_synced_at, cpm.updated_at) AS last_activity_at,
	       (SELECT COUNT(*) FROM ecom_channel_listing_events ev
	         WHERE ev.product_id = cpm.product_id AND ev.connection_id = cpm.connection_id
	           AND ev.vehicle_fitment_key = COALESCE(cpm.vehicle_fitment_id, 0)) AS attempts
	FROM ecom_channel_product_map cpm
	JOIN ecom_channel_connections cc ON cc.id = cpm.connection_id
	JOIN ecom_channels ch ON ch.id = cc.channel_id
	JOIN ecom_products p ON p.id = cpm.product_id
	WHERE cpm.deleted_at IS NULL AND UPPER(TRIM(ch.code)) = UPPER(TRIM(?))

	UNION ALL

	SELECT 'not_published', e.id, e.product_id, p.sku, p.name,
	       e.connection_id, cc.name, e.vehicle_fitment_id,
	       NULL, NULL, NULL,
	       NULL, NULL,
	       'not_published',
	       e.message,
	       e.created_at,
	       last.attempts
	FROM (
	    SELECT product_id, connection_id, vehicle_fitment_key, MAX(id) AS max_id, COUNT(*) AS attempts
	    FROM ecom_channel_listing_events
	    WHERE action = 'publish'
	    GROUP BY product_id, connection_id, vehicle_fitment_key
	) last
	JOIN ecom_channel_listing_events e ON e.id = last.max_id
	JOIN ecom_channel_connections cc ON cc.id = e.connection_id
	JOIN ecom_channels ch ON ch.id = cc.channel_id
	JOIN ecom_products p ON p.id = e.product_id
	WHERE e.outcome = 'error' AND UPPER(TRIM(ch.code)) = UPPER(TRIM(?))
	  AND NOT EXISTS (
	      SELECT 1 FROM ecom_channel_product_map cpm
	      WHERE cpm.product_id = e.product_id AND cpm.connection_id = e.connection_id
	        AND COALESCE(cpm.vehicle_fitment_id, 0) = e.vehicle_fitment_key
	        AND cpm.deleted_at IS NULL AND cpm.status <> 'closed'
	  )
`

// FindListingsView returns one page of the combined listings view plus
// per-state counts (counts honor connection/search filters, not state).
func (r *ChannelListingEventsRepository) FindListingsView(ctx context.Context, f ListingViewFilter) (*PaginatedListingView, error) {
	if f.PageSize <= 0 {
		f.PageSize = 50
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	baseWhere := []string{"1=1"}
	baseArgs := []any{f.ChannelCode, f.ChannelCode}
	if f.ConnectionID > 0 {
		baseWhere = append(baseWhere, "x.connection_id = ?")
		baseArgs = append(baseArgs, f.ConnectionID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		baseWhere = append(baseWhere, "(x.sku LIKE ? OR x.product_name LIKE ? OR x.external_id LIKE ? OR x.listing_title LIKE ?)")
		baseArgs = append(baseArgs, like, like, like, like)
	}

	countsQuery := `SELECT x.state, COUNT(*), SUM(x.last_error IS NOT NULL) FROM (` + listingViewSource + `) x WHERE ` +
		strings.Join(baseWhere, " AND ") + ` GROUP BY x.state`
	countRows, err := r.db.QueryContext(ctx, countsQuery, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("error counting listings view: %w", err)
	}
	stateCounts := map[string]int64{}
	var withError int64
	for countRows.Next() {
		var state string
		var n int64
		var errN sql.NullInt64
		if err := countRows.Scan(&state, &n, &errN); err != nil {
			countRows.Close()
			return nil, fmt.Errorf("error scanning listings view counts: %w", err)
		}
		stateCounts[state] = n
		withError += errN.Int64
	}
	countRows.Close()

	where := append([]string{}, baseWhere...)
	args := append([]any{}, baseArgs...)
	if f.State != "" {
		where = append(where, "x.state = ?")
		args = append(args, f.State)
	}
	if f.WithError {
		where = append(where, "x.last_error IS NOT NULL")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+listingViewSource+`) x WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting filtered listings view: %w", err)
	}

	pageArgs := append(append([]any{}, args...), f.PageSize, f.Offset)
	rows, err := r.db.QueryContext(ctx, `SELECT x.kind, x.id, x.product_id, x.sku, x.product_name, x.connection_id,
		x.connection_name, x.vehicle_fitment_id, x.listing_title, x.external_id, x.external_category_id,
		x.raw_status, x.paused_reason, x.state, x.last_error, x.last_activity_at, x.attempts
		FROM (`+listingViewSource+`) x WHERE `+whereSQL+`
		ORDER BY x.last_activity_at DESC, x.id DESC LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("error querying listings view: %w", err)
	}
	defer rows.Close()

	listings := make([]ListingViewRow, 0)
	for rows.Next() {
		var v ListingViewRow
		var fitment sql.NullInt64
		var title, externalID, externalCat, rawStatus, pausedReason, lastError sql.NullString
		var lastActivity sql.NullTime
		if err := rows.Scan(&v.Kind, &v.ID, &v.ProductID, &v.SKU, &v.ProductName, &v.ConnectionID,
			&v.ConnectionName, &fitment, &title, &externalID, &externalCat,
			&rawStatus, &pausedReason, &v.State, &lastError, &lastActivity, &v.Attempts); err != nil {
			return nil, fmt.Errorf("error scanning listings view row: %w", err)
		}
		if fitment.Valid {
			v.VehicleFitmentID = &fitment.Int64
		}
		if title.Valid {
			v.ListingTitle = &title.String
		}
		if externalID.Valid {
			v.ExternalID = &externalID.String
		}
		if externalCat.Valid {
			v.ExternalCategoryID = &externalCat.String
		}
		if rawStatus.Valid {
			v.RawStatus = &rawStatus.String
		}
		if pausedReason.Valid {
			v.PausedReason = &pausedReason.String
		}
		if lastError.Valid {
			v.LastError = &lastError.String
		}
		if lastActivity.Valid {
			v.LastActivityAt = &lastActivity.Time
		}
		listings = append(listings, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listings view: %w", err)
	}

	return &PaginatedListingView{
		Total:        total,
		Offset:       f.Offset,
		PageSize:     f.PageSize,
		Listings:     listings,
		StateCounts:  stateCounts,
		WithErrorCnt: withError,
	}, nil
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrChannelProductMapNotFound = errors.New("channel product map not found")

// ChannelProductMapDTO is a single external listing a local product resolves
// to on a channel connection. VehicleFitmentID is nil for a "general"
// listing (no vehicle compatibility involved — e.g. apparel, or any listing
// on a connection that never lists more than once per product) and points
// at a specific ecom_vehicle_fitments row when the listing exists because of
// that particular compatibility (only meaningful on connections where
// ecom_channel_connections.allows_multiple_listings is true).
type ChannelProductMapDTO struct {
	ID               int64   `json:"id"`
	ListingTitle     *string `json:"listingTitle,omitempty"`
	ProductID        int64   `json:"productId"`
	ConnectionID     int64   `json:"connectionId"`
	VehicleFitmentID *int64  `json:"vehicleFitmentId,omitempty"`
	IsEnabled        bool    `json:"isEnabled"`
	ExternalID       *string `json:"externalId,omitempty"`
	// ExternalCategoryID is the marketplace's own category id the listing was
	// created under (e.g. MercadoLibre's category_id) — kept here so it can be
	// diffed against ecom_channel_category_map without re-fetching the
	// listing from the marketplace.
	ExternalCategoryID *string    `json:"externalCategoryId,omitempty"`
	Status             string     `json:"status"`
	LastSyncedAt       *time.Time `json:"lastSyncedAt,omitempty"`
	LastError          *string    `json:"lastError,omitempty"`
	CreatedBy          int64      `json:"createdBy"`
	UpdatedBy          *int64     `json:"updatedBy,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
}

// UpsertChannelProductMapInput records (or refreshes) the external id a
// local product resolves to on a given connection, for a given vehicle
// fitment, once a sync succeeds. ecom_channel_product_map has a unique key
// on (product_id, connection_id, vehicle_fitment_key) — vehicle_fitment_key
// coalesces a nil VehicleFitmentID to 0, so a product still only ever maps
// to one external id per connection when VehicleFitmentID is nil, while
// still allowing one row per compatibility when it isn't.
type UpsertChannelProductMapInput struct {
	ProductID        int64
	ConnectionID     int64
	VehicleFitmentID *int64
	ListingTitle     string
	ExternalID       string
	// ExternalCategoryID is optional; pass the current value back through
	// (e.g. derefString(existing.ExternalCategoryID)) to leave it untouched
	// on calls — like a price/stock refresh — that don't resolve a category.
	ExternalCategoryID string
	Status             string
	ActorID            int64
}

type ChannelProductMapRepository struct {
	db Querier
}

func NewChannelProductMapRepository(db Querier) *ChannelProductMapRepository {
	return &ChannelProductMapRepository{db: db}
}

const channelProductMapColumns = `
	id, listing_title, product_id, connection_id, vehicle_fitment_id, is_enabled, external_id, external_category_id, status,
	last_synced_at, last_error, created_by, updated_by, created_at, updated_at, deleted_at
`

// FindByProductAndConnection finds a product's general (non-fitment)
// listing on a connection — the row every Odoo-style single-listing sync
// reads and writes.
func (r *ChannelProductMapRepository) FindByProductAndConnection(ctx context.Context, productID, connectionID int64) (*ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE product_id = ? AND connection_id = ? AND vehicle_fitment_id IS NULL AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelProductMapRow(r.db.QueryRowContext(ctx, query, productID, connectionID))
}

// FindByProductConnectionAndFitment finds the listing for one specific
// (product, connection, vehicle fitment) combination. vehicleFitmentID nil
// looks up the general listing, same as FindByProductAndConnection.
func (r *ChannelProductMapRepository) FindByProductConnectionAndFitment(ctx context.Context, productID, connectionID int64, vehicleFitmentID *int64) (*ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE product_id = ? AND connection_id = ? AND vehicle_fitment_id <=> ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelProductMapRow(r.db.QueryRowContext(ctx, query, productID, connectionID, vehicleFitmentID))
}

// FindAllByProductAndConnection returns every listing a product has on a
// connection — one row for the general listing, or one per vehicle fitment
// on connections where allows_multiple_listings is true.
func (r *ChannelProductMapRepository) FindAllByProductAndConnection(ctx context.Context, productID, connectionID int64) ([]ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE product_id = ? AND connection_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, productID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error querying channel product map for product %d, connection %d: %w", productID, connectionID, err)
	}
	defer rows.Close()

	return scanChannelProductMapRows(rows)
}

// FindAllByConnectionID returns every listing recorded on a connection,
// across every product — used to refresh marketplace status/price/stock in
// bulk without touching other connections' listings.
func (r *ChannelProductMapRepository) FindAllByConnectionID(ctx context.Context, connectionID int64) ([]ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE connection_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error querying channel product map for connection %d: %w", connectionID, err)
	}
	defer rows.Close()

	return scanChannelProductMapRows(rows)
}

// FindByConnectionAndExternalID resolves the local product a marketplace
// listing id maps to on a connection — used when a caller only has the
// external item id (e.g. copying compatibilities between two MercadoLibre
// items by their item ids) and needs the corresponding ecom_products row.
func (r *ChannelProductMapRepository) FindByConnectionAndExternalID(ctx context.Context, connectionID int64, externalID string) (*ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE connection_id = ? AND external_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`

	return scanChannelProductMapRow(r.db.QueryRowContext(ctx, query, connectionID, externalID))
}

func (r *ChannelProductMapRepository) findByID(ctx context.Context, id int64) (*ChannelProductMapDTO, error) {
	query := `
		SELECT ` + channelProductMapColumns + `
		FROM ecom_channel_product_map
		WHERE id = ? AND deleted_at IS NULL
	`

	return scanChannelProductMapRow(r.db.QueryRowContext(ctx, query, id))
}

// Upsert creates the mapping row for a (product, connection, vehicle
// fitment) combination on first successful sync, or refreshes its external
// id/status/last_synced_at on subsequent ones.
func (r *ChannelProductMapRepository) Upsert(ctx context.Context, input UpsertChannelProductMapInput) (*ChannelProductMapDTO, error) {
	existing, err := r.FindByProductConnectionAndFitment(ctx, input.ProductID, input.ConnectionID, input.VehicleFitmentID)
	if err != nil && !errors.Is(err, ErrChannelProductMapNotFound) {
		return nil, fmt.Errorf("error loading channel product map: %w", err)
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_channel_product_map
				(product_id, connection_id, vehicle_fitment_id, listing_title, external_id, external_category_id, status, last_synced_at, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), ?)
		`

		result, err := r.db.ExecContext(ctx,
			query,
			input.ProductID,
			input.ConnectionID,
			input.VehicleFitmentID,
			nullableString(input.ListingTitle),
			input.ExternalID,
			nullableString(input.ExternalCategoryID),
			input.Status,
			input.ActorID,
		)
		if err != nil {
			return nil, fmt.Errorf("error creating channel product map: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(ctx, id)
	}

	query := `
		UPDATE ecom_channel_product_map
		SET listing_title = ?, external_id = ?, external_category_id = ?, status = ?, last_synced_at = NOW(), last_error = NULL, updated_by = ?
		WHERE id = ?
	`

	if _, err := r.db.ExecContext(ctx, query, nullableString(input.ListingTitle), input.ExternalID, nullableString(input.ExternalCategoryID), input.Status, input.ActorID, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating channel product map: %w", err)
	}

	return r.findByID(ctx, existing.ID)
}

// UpdateStatus persists a refreshed status for an already-known row (e.g.
// after polling the marketplace) without touching its title or external id.
func (r *ChannelProductMapRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE ecom_channel_product_map SET status = ?, last_synced_at = NOW() WHERE id = ?`

	if _, err := r.db.ExecContext(ctx, query, status, id); err != nil {
		return fmt.Errorf("error updating channel product map status: %w", err)
	}

	return nil
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func scanChannelProductMapRow(row *sql.Row) (*ChannelProductMapDTO, error) {
	m, err := scanChannelProductMap(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelProductMapNotFound
		}
		return nil, err
	}

	return &m, nil
}

func scanChannelProductMapRows(rows *sql.Rows) ([]ChannelProductMapDTO, error) {
	maps := make([]ChannelProductMapDTO, 0)
	for rows.Next() {
		m, err := scanChannelProductMap(rows)
		if err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel product map rows: %w", err)
	}

	return maps, nil
}

func scanChannelProductMap(scanner interface{ Scan(dest ...any) error }) (ChannelProductMapDTO, error) {
	var m ChannelProductMapDTO
	var listingTitle sql.NullString
	var vehicleFitmentID sql.NullInt64
	var externalID sql.NullString
	var externalCategoryID sql.NullString
	var lastSyncedAt sql.NullTime
	var lastError sql.NullString
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&m.ID,
		&listingTitle,
		&m.ProductID,
		&m.ConnectionID,
		&vehicleFitmentID,
		&m.IsEnabled,
		&externalID,
		&externalCategoryID,
		&m.Status,
		&lastSyncedAt,
		&lastError,
		&m.CreatedBy,
		&updatedBy,
		&m.CreatedAt,
		&m.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return ChannelProductMapDTO{}, fmt.Errorf("error scanning channel product map: %w", err)
	}

	if listingTitle.Valid {
		m.ListingTitle = &listingTitle.String
	}
	if vehicleFitmentID.Valid {
		m.VehicleFitmentID = &vehicleFitmentID.Int64
	}
	if externalID.Valid {
		m.ExternalID = &externalID.String
	}
	if externalCategoryID.Valid {
		m.ExternalCategoryID = &externalCategoryID.String
	}
	if lastSyncedAt.Valid {
		m.LastSyncedAt = &lastSyncedAt.Time
	}
	if lastError.Valid {
		m.LastError = &lastError.String
	}
	if updatedBy.Valid {
		m.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		m.DeletedAt = &deletedAt.Time
	}

	return m, nil
}

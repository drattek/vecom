package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrChannelCategoryMapNotFound = errors.New("channel category map not found")

type ChannelCategoryMapDTO struct {
	ID                   int64      `json:"id"`
	CategoryID           int64      `json:"categoryId"`
	ConnectionID         int64      `json:"connectionId"`
	ExternalCategoryID   string     `json:"externalCategoryId"`
	ExternalCategoryName *string    `json:"externalCategoryName,omitempty"`
	CreatedBy            int64      `json:"createdBy"`
	UpdatedBy            *int64     `json:"updatedBy,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
}

// UpsertChannelCategoryMapInput links a local ecom_categories row to the
// category id/name it corresponds to in a marketplace/connection (e.g. an
// Odoo product.public.category id). ecom_channel_category_map has a unique
// key on (category_id, connection_id), so a given local category can only
// map to one external category per connection.
type UpsertChannelCategoryMapInput struct {
	CategoryID           int64
	ConnectionID         int64
	ExternalCategoryID   string
	ExternalCategoryName *string
	ActorID              int64
}

type ChannelCategoryMapRepository struct {
	db Querier
}

func NewChannelCategoryMapRepository(db Querier) *ChannelCategoryMapRepository {
	return &ChannelCategoryMapRepository{db: db}
}

func (r *ChannelCategoryMapRepository) FindByCategoryAndConnection(ctx context.Context, categoryID, connectionID int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE category_id = ? AND connection_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelCategoryMapRow(r.db.QueryRowContext(ctx, query, categoryID, connectionID))
}

// FindByExternalCategoryAndConnection finds the mapping row for a
// marketplace's own category id on a connection — the reverse lookup of
// FindByCategoryAndConnection, used to tell whether a category MercadoLibre
// (or another channel) just returned has already been replicated locally for
// this connection.
func (r *ChannelCategoryMapRepository) FindByExternalCategoryAndConnection(ctx context.Context, externalCategoryID string, connectionID int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE external_category_id = ? AND connection_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelCategoryMapRow(r.db.QueryRowContext(ctx, query, externalCategoryID, connectionID))
}

// FindActiveConnectionIDsByCategory returns the ids of every active, non-
// deleted channel connection that has a category mapping for categoryID —
// i.e. the connections a product in that local category may be auto-published
// to (see ListingDiscoveryScheduler). Ordered by connection id; always a
// slice, empty when the category is mapped nowhere.
func (r *ChannelCategoryMapRepository) FindActiveConnectionIDsByCategory(ctx context.Context, categoryID int64) ([]int64, error) {
	query := `
		SELECT ccm.connection_id
		FROM ecom_channel_category_map ccm
		JOIN ecom_channel_connections cc ON cc.id = ccm.connection_id
		WHERE ccm.category_id = ? AND ccm.deleted_at IS NULL
		  AND cc.deleted_at IS NULL AND cc.status = 'active'
		ORDER BY ccm.connection_id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("error querying connections for category %d: %w", categoryID, err)
	}
	defer rows.Close()

	connectionIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("error scanning connection id for category %d: %w", categoryID, err)
		}
		connectionIDs = append(connectionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating connections for category %d: %w", categoryID, err)
	}

	return connectionIDs, nil
}

func (r *ChannelCategoryMapRepository) findByID(ctx context.Context, id int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE id = ? AND deleted_at IS NULL
	`

	return scanChannelCategoryMapRow(r.db.QueryRowContext(ctx, query, id))
}

// Upsert creates the mapping row for a (category, connection) pair on first
// write, or updates the external category id/name on subsequent writes (so
// re-running a migration keeps the mapping current instead of erroring on
// the unique key).
func (r *ChannelCategoryMapRepository) Upsert(ctx context.Context, input UpsertChannelCategoryMapInput) (*ChannelCategoryMapDTO, error) {
	existing, err := r.FindByCategoryAndConnection(ctx, input.CategoryID, input.ConnectionID)
	if err != nil && !errors.Is(err, ErrChannelCategoryMapNotFound) {
		return nil, fmt.Errorf("error loading channel category map: %w", err)
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_channel_category_map
				(category_id, connection_id, external_category_id, external_category_name, created_by)
			VALUES (?, ?, ?, ?, ?)
		`

		result, err := r.db.ExecContext(ctx, query, input.CategoryID, input.ConnectionID, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID)
		if err != nil {
			return nil, fmt.Errorf("error creating channel category map: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(ctx, id)
	}

	query := `
		UPDATE ecom_channel_category_map
		SET external_category_id = ?, external_category_name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	if _, err := r.db.ExecContext(ctx, query, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating channel category map: %w", err)
	}

	return r.findByID(ctx, existing.ID)
}

func scanChannelCategoryMapRow(row *sql.Row) (*ChannelCategoryMapDTO, error) {
	var m ChannelCategoryMapDTO
	err := row.Scan(
		&m.ID,
		&m.CategoryID,
		&m.ConnectionID,
		&m.ExternalCategoryID,
		&m.ExternalCategoryName,
		&m.CreatedBy,
		&m.UpdatedBy,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelCategoryMapNotFound
		}
		return nil, fmt.Errorf("error scanning channel category map: %w", err)
	}

	return &m, nil
}

package mysql

import (
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
	db *sql.DB
}

func NewChannelCategoryMapRepository(db *sql.DB) *ChannelCategoryMapRepository {
	return &ChannelCategoryMapRepository{db: db}
}

func (r *ChannelCategoryMapRepository) FindByCategoryAndConnection(categoryID, connectionID int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE category_id = ? AND connection_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelCategoryMapRow(r.db.QueryRow(query, categoryID, connectionID))
}

// FindByExternalCategoryAndConnection finds the mapping row for a
// marketplace's own category id on a connection — the reverse lookup of
// FindByCategoryAndConnection, used to tell whether a category MercadoLibre
// (or another channel) just returned has already been replicated locally for
// this connection.
func (r *ChannelCategoryMapRepository) FindByExternalCategoryAndConnection(externalCategoryID string, connectionID int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE external_category_id = ? AND connection_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelCategoryMapRow(r.db.QueryRow(query, externalCategoryID, connectionID))
}

func (r *ChannelCategoryMapRepository) findByID(id int64) (*ChannelCategoryMapDTO, error) {
	query := `
		SELECT id, category_id, connection_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_category_map
		WHERE id = ? AND deleted_at IS NULL
	`

	return scanChannelCategoryMapRow(r.db.QueryRow(query, id))
}

// Upsert creates the mapping row for a (category, connection) pair on first
// write, or updates the external category id/name on subsequent writes (so
// re-running a migration keeps the mapping current instead of erroring on
// the unique key).
func (r *ChannelCategoryMapRepository) Upsert(input UpsertChannelCategoryMapInput) (*ChannelCategoryMapDTO, error) {
	existing, err := r.FindByCategoryAndConnection(input.CategoryID, input.ConnectionID)
	if err != nil && !errors.Is(err, ErrChannelCategoryMapNotFound) {
		return nil, fmt.Errorf("error loading channel category map: %w", err)
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_channel_category_map
				(category_id, connection_id, external_category_id, external_category_name, created_by)
			VALUES (?, ?, ?, ?, ?)
		`

		result, err := r.db.Exec(query, input.CategoryID, input.ConnectionID, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID)
		if err != nil {
			return nil, fmt.Errorf("error creating channel category map: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(id)
	}

	query := `
		UPDATE ecom_channel_category_map
		SET external_category_id = ?, external_category_name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	if _, err := r.db.Exec(query, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating channel category map: %w", err)
	}

	return r.findByID(existing.ID)
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

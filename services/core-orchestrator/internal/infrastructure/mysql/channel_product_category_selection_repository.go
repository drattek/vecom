package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrChannelProductCategorySelectionNotFound = errors.New("channel product category selection not found")

// ChannelProductCategorySelectionDTO records, for one (product, connection)
// pair, the external category the user picked in the "Sincronización" tab
// before the product has any listing yet on that connection — see ADR 0005.
// CategoryID is the local ecom_categories leaf category_import.Service.Import
// resolved/created for ExternalCategoryID, kept here only so callers that
// need a local category id (attribute slot scoping, Odoo's category push)
// don't have to re-derive it; it is never written to ecom_products.category_id.
type ChannelProductCategorySelectionDTO struct {
	ID                   int64      `json:"id"`
	ProductID            int64      `json:"productId"`
	ConnectionID         int64      `json:"connectionId"`
	CategoryID           int64      `json:"categoryId"`
	ExternalCategoryID   string     `json:"externalCategoryId"`
	ExternalCategoryName *string    `json:"externalCategoryName,omitempty"`
	CreatedBy            int64      `json:"createdBy"`
	UpdatedBy            *int64     `json:"updatedBy,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
}

// UpsertChannelProductCategorySelectionInput writes the (product, connection)
// selection. ecom_channel_product_category_selection has a unique key on
// (product_id, connection_id), so a product can only have one pending
// category choice per connection — choosing again before publishing replaces
// it rather than erroring.
type UpsertChannelProductCategorySelectionInput struct {
	ProductID            int64
	ConnectionID         int64
	CategoryID           int64
	ExternalCategoryID   string
	ExternalCategoryName *string
	ActorID              int64
}

type ChannelProductCategorySelectionRepository struct {
	db Querier
}

func NewChannelProductCategorySelectionRepository(db Querier) *ChannelProductCategorySelectionRepository {
	return &ChannelProductCategorySelectionRepository{db: db}
}

func (r *ChannelProductCategorySelectionRepository) FindByProductAndConnection(ctx context.Context, productID, connectionID int64) (*ChannelProductCategorySelectionDTO, error) {
	query := `
		SELECT id, product_id, connection_id, category_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_product_category_selection
		WHERE product_id = ? AND connection_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelProductCategorySelectionRow(r.db.QueryRowContext(ctx, query, productID, connectionID))
}

func (r *ChannelProductCategorySelectionRepository) findByID(ctx context.Context, id int64) (*ChannelProductCategorySelectionDTO, error) {
	query := `
		SELECT id, product_id, connection_id, category_id, external_category_id, external_category_name,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_product_category_selection
		WHERE id = ? AND deleted_at IS NULL
	`

	return scanChannelProductCategorySelectionRow(r.db.QueryRowContext(ctx, query, id))
}

// Upsert creates the selection row for a (product, connection) pair on first
// write, or replaces the chosen category on a later write — e.g. the user
// picks the predictor's suggestion, then changes their mind and picks another
// category from the tree before ever publishing.
func (r *ChannelProductCategorySelectionRepository) Upsert(ctx context.Context, input UpsertChannelProductCategorySelectionInput) (*ChannelProductCategorySelectionDTO, error) {
	existing, err := r.FindByProductAndConnection(ctx, input.ProductID, input.ConnectionID)
	if err != nil && !errors.Is(err, ErrChannelProductCategorySelectionNotFound) {
		return nil, fmt.Errorf("error loading channel product category selection: %w", err)
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_channel_product_category_selection
				(product_id, connection_id, category_id, external_category_id, external_category_name, created_by)
			VALUES (?, ?, ?, ?, ?, ?)
		`

		result, err := r.db.ExecContext(ctx, query, input.ProductID, input.ConnectionID, input.CategoryID, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID)
		if err != nil {
			return nil, fmt.Errorf("error creating channel product category selection: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(ctx, id)
	}

	query := `
		UPDATE ecom_channel_product_category_selection
		SET category_id = ?, external_category_id = ?, external_category_name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	if _, err := r.db.ExecContext(ctx, query, input.CategoryID, input.ExternalCategoryID, input.ExternalCategoryName, input.ActorID, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating channel product category selection: %w", err)
	}

	return r.findByID(ctx, existing.ID)
}

func scanChannelProductCategorySelectionRow(row *sql.Row) (*ChannelProductCategorySelectionDTO, error) {
	var m ChannelProductCategorySelectionDTO
	err := row.Scan(
		&m.ID,
		&m.ProductID,
		&m.ConnectionID,
		&m.CategoryID,
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
			return nil, ErrChannelProductCategorySelectionNotFound
		}
		return nil, fmt.Errorf("error scanning channel product category selection: %w", err)
	}

	return &m, nil
}

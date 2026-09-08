package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrProductMediaNotFound = errors.New("product media not found")

// ecom_product_media has no surrogate key — a row is identified by
// (product_id, file_id), which is unique per product in practice (a file is
// attached once). These write helpers keep that invariant: Create upserts,
// SoftDelete/reads key on the pair.
type ProductMediaDTO struct {
	ProductID int64      `json:"productId"`
	Type      string     `json:"type"`
	FileID    int64      `json:"fileId"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type CreateProductMediaInput struct {
	ProductID int64
	Type      string
	FileID    int64
	CreatedBy int64
}

type ProductMediaRepository struct {
	db Querier
}

func NewProductMediaRepository(db Querier) *ProductMediaRepository {
	return &ProductMediaRepository{db: db}
}

// FindByProductAndFile looks up the row linking a file to a product, including
// soft-deleted rows (deletedAt is populated) so callers can revive one instead
// of inserting a duplicate.
func (r *ProductMediaRepository) FindByProductAndFile(ctx context.Context, productID, fileID int64) (*ProductMediaDTO, error) {
	query := `
		SELECT product_id, type, file_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_media
		WHERE product_id = ? AND file_id = ?
		LIMIT 1
	`

	var (
		media     ProductMediaDTO
		updatedBy sql.NullInt64
		deletedAt sql.NullTime
	)
	if err := r.db.QueryRowContext(ctx, query, productID, fileID).Scan(
		&media.ProductID, &media.Type, &media.FileID, &media.CreatedBy, &updatedBy,
		&media.CreatedAt, &media.UpdatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductMediaNotFound
		}
		return nil, err
	}

	if updatedBy.Valid {
		media.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		media.DeletedAt = &deletedAt.Time
	}
	return &media, nil
}

// Create upserts the (product_id, file_id) link: a pre-existing row (even
// soft-deleted) is revived and its type refreshed rather than duplicated —
// the table has no unique key to lean on.
func (r *ProductMediaRepository) Create(ctx context.Context, input CreateProductMediaInput) (*ProductMediaDTO, error) {
	existing, err := r.FindByProductAndFile(ctx, input.ProductID, input.FileID)
	if err != nil && !errors.Is(err, ErrProductMediaNotFound) {
		return nil, err
	}

	if existing != nil {
		update := `
			UPDATE ecom_product_media
			SET type = ?, deleted_at = NULL, updated_by = ?, updated_at = NOW()
			WHERE product_id = ? AND file_id = ?
		`
		if _, err := r.db.ExecContext(ctx, update, input.Type, input.CreatedBy, input.ProductID, input.FileID); err != nil {
			return nil, err
		}
		return r.FindByProductAndFile(ctx, input.ProductID, input.FileID)
	}

	insert := `
		INSERT INTO ecom_product_media (product_id, type, file_id, created_by)
		VALUES (?, ?, ?, ?)
	`
	if _, err := r.db.ExecContext(ctx, insert, input.ProductID, input.Type, input.FileID, input.CreatedBy); err != nil {
		return nil, err
	}
	return r.FindByProductAndFile(ctx, input.ProductID, input.FileID)
}

func (r *ProductMediaRepository) SoftDelete(ctx context.Context, productID, fileID, updatedBy int64) error {
	query := `
		UPDATE ecom_product_media
		SET deleted_at = NOW(), updated_by = ?
		WHERE product_id = ? AND file_id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, updatedBy, productID, fileID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrProductMediaNotFound
	}
	return nil
}

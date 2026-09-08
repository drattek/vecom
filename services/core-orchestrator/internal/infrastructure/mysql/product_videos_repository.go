package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductVideoNotFound = errors.New("product video not found")
)

type ProductVideoDTO struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"productId"`
	FileID    int64     `json:"fileId"`
	CreatedBy int64     `json:"createdBy"`
	UpdatedBy *int64    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PaginatedProductVideos struct {
	Data  []ProductVideoDTO `json:"data"`
	Total int               `json:"total"`
}

type CreateProductVideoInput struct {
	ProductID int64
	FileID    int64
	CreatedBy int64
}

type UpdateProductVideoInput struct {
	UpdatedBy int64
}

type ProductVideosRepository struct {
	db Querier
}

func NewProductVideosRepository(db Querier) *ProductVideosRepository {
	return &ProductVideosRepository{db: db}
}

func (r *ProductVideosRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedProductVideos, error) {
	query := `
		SELECT id, product_id, file_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_videos
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []ProductVideoDTO
	for rows.Next() {
		var p ProductVideoDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.FileID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		videos = append(videos, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_videos WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductVideos{Data: videos, Total: total}, nil
}

func (r *ProductVideosRepository) FindByID(ctx context.Context, id int64) (*ProductVideoDTO, error) {
	query := `
		SELECT id, product_id, file_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_videos
		WHERE id = ? AND deleted_at IS NULL
	`

	var p ProductVideoDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.ProductID, &p.FileID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductVideoNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductVideosRepository) FindByProductID(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedProductVideos, error) {
	query := `
		SELECT id, product_id, file_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_videos
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, productID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []ProductVideoDTO
	for rows.Next() {
		var p ProductVideoDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.FileID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		videos = append(videos, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_videos WHERE product_id = ? AND deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, productID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductVideos{Data: videos, Total: total}, nil
}

// FindByProductAndFile looks up an existing ecom_product_videos row linking a
// file to a product, so callers that receive the same URL twice can reuse the
// existing row instead of inserting a duplicate.
func (r *ProductVideosRepository) FindByProductAndFile(ctx context.Context, productID, fileID int64) (*ProductVideoDTO, error) {
	query := `
		SELECT id, product_id, file_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_videos
		WHERE product_id = ? AND file_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	var p ProductVideoDTO
	if err := r.db.QueryRowContext(ctx, query, productID, fileID).Scan(&p.ID, &p.ProductID, &p.FileID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductVideoNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductVideosRepository) Create(ctx context.Context, input CreateProductVideoInput) (*ProductVideoDTO, error) {
	query := `
		INSERT INTO ecom_product_videos (product_id, file_id, created_by)
		VALUES (?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.FileID, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *ProductVideosRepository) Update(ctx context.Context, id int64, input UpdateProductVideoInput) (*ProductVideoDTO, error) {
	query := `
		UPDATE ecom_product_videos
		SET updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductVideoNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ProductVideosRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_product_videos SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductVideoNotFound
	}

	return nil
}

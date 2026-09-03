package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductImageNotFound = errors.New("product image not found")
)

type ProductImageDTO struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"productId"`
	FileID    int64     `json:"fileId"`
	IsFirst   bool      `json:"isFirst"`
	CreatedBy int64     `json:"createdBy"`
	UpdatedBy *int64    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PaginatedProductImages struct {
	Data  []ProductImageDTO `json:"data"`
	Total int               `json:"total"`
}

type CreateProductImageInput struct {
	ProductID int64
	FileID    int64
	IsFirst   bool
	CreatedBy int64
}

type UpdateProductImageInput struct {
	IsFirst   bool
	UpdatedBy int64
}

type ProductImagesRepository struct {
	db Querier
}

func NewProductImagesRepository(db Querier) *ProductImagesRepository {
	return &ProductImagesRepository{db: db}
}

func (r *ProductImagesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedProductImages, error) {
	query := `
		SELECT id, product_id, file_id, is_first, created_by, updated_by, created_at, updated_at
		FROM ecom_product_images
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ProductImageDTO
	for rows.Next() {
		var p ProductImageDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.FileID, &p.IsFirst, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		images = append(images, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_images WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductImages{Data: images, Total: total}, nil
}

func (r *ProductImagesRepository) FindByID(ctx context.Context, id int64) (*ProductImageDTO, error) {
	query := `
		SELECT id, product_id, file_id, is_first, created_by, updated_by, created_at, updated_at
		FROM ecom_product_images
		WHERE id = ? AND deleted_at IS NULL
	`

	var p ProductImageDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.ProductID, &p.FileID, &p.IsFirst, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductImageNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductImagesRepository) FindByProductID(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedProductImages, error) {
	query := `
		SELECT id, product_id, file_id, is_first, created_by, updated_by, created_at, updated_at
		FROM ecom_product_images
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, productID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ProductImageDTO
	for rows.Next() {
		var p ProductImageDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.FileID, &p.IsFirst, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		images = append(images, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_images WHERE product_id = ? AND deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, productID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductImages{Data: images, Total: total}, nil
}

// FindAllByProductID returns every image for a product, cover image
// (is_first) first, with no pagination — for flows that need the complete
// set (e.g. building a marketplace sync payload) rather than a page of it.
func (r *ProductImagesRepository) FindAllByProductID(ctx context.Context, productID int64) ([]ProductImageDTO, error) {
	query := `
		SELECT id, product_id, file_id, is_first, created_by, updated_by, created_at, updated_at
		FROM ecom_product_images
		WHERE product_id = ? AND deleted_at IS NULL
		ORDER BY is_first DESC, id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]ProductImageDTO, 0)
	for rows.Next() {
		var p ProductImageDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.FileID, &p.IsFirst, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		images = append(images, p)
	}

	return images, rows.Err()
}

// FindByProductAndFile looks up an existing ecom_product_images row linking
// a product to a file, so callers can tell whether a given file is already
// attached to the product before inserting a duplicate link.
func (r *ProductImagesRepository) FindByProductAndFile(ctx context.Context, productID, fileID int64) (*ProductImageDTO, error) {
	query := `
		SELECT id, product_id, file_id, is_first, created_by, updated_by, created_at, updated_at
		FROM ecom_product_images
		WHERE product_id = ? AND file_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	var p ProductImageDTO
	if err := r.db.QueryRowContext(ctx, query, productID, fileID).Scan(&p.ID, &p.ProductID, &p.FileID, &p.IsFirst, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductImageNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductImagesRepository) Create(ctx context.Context, input CreateProductImageInput) (*ProductImageDTO, error) {
	query := `
		INSERT INTO ecom_product_images (product_id, file_id, is_first, created_by)
		VALUES (?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.FileID, input.IsFirst, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *ProductImagesRepository) Update(ctx context.Context, id int64, input UpdateProductImageInput) (*ProductImageDTO, error) {
	query := `
		UPDATE ecom_product_images
		SET is_first = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.IsFirst, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductImageNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ProductImagesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_product_images SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductImageNotFound
	}

	return nil
}

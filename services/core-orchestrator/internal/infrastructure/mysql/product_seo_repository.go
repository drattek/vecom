package mysql

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductSEONotFound = errors.New("product seo not found")
)

type ProductSEODTO struct {
	ProductID       int64     `json:"productId"`
	MetaTitle       *string   `json:"metaTitle"`
	MetaDescription *string   `json:"metaDescription"`
	Keywords        *string   `json:"keywords"`
	CreatedBy       int64     `json:"createdBy"`
	UpdatedBy       *int64    `json:"updatedBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type CreateProductSEOInput struct {
	ProductID       int64
	MetaTitle       *string
	MetaDescription *string
	Keywords        *string
	CreatedBy       int64
}

type UpdateProductSEOInput struct {
	MetaTitle       *string
	MetaDescription *string
	Keywords        *string
	UpdatedBy       int64
}

type ProductSEORepository struct {
	db *sql.DB
}

func NewProductSEORepository(db *sql.DB) *ProductSEORepository {
	return &ProductSEORepository{db: db}
}

func (r *ProductSEORepository) FindByProductID(productID int64) (*ProductSEODTO, error) {
	query := `
		SELECT product_id, meta_title, meta_description, keywords, created_by, updated_by, created_at, updated_at
		FROM ecom_product_seo
		WHERE product_id = ? AND deleted_at IS NULL
	`

	var p ProductSEODTO
	if err := r.db.QueryRow(query, productID).Scan(&p.ProductID, &p.MetaTitle, &p.MetaDescription, &p.Keywords, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductSEONotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductSEORepository) Create(input CreateProductSEOInput) (*ProductSEODTO, error) {
	query := `
		INSERT INTO ecom_product_seo (product_id, meta_title, meta_description, keywords, created_by)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, input.ProductID, input.MetaTitle, input.MetaDescription, input.Keywords, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	return r.FindByProductID(input.ProductID)
}

func (r *ProductSEORepository) Update(productID int64, input UpdateProductSEOInput) (*ProductSEODTO, error) {
	query := `
		UPDATE ecom_product_seo
		SET meta_title = ?, meta_description = ?, keywords = ?, updated_by = ?, updated_at = NOW()
		WHERE product_id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.MetaTitle, input.MetaDescription, input.Keywords, input.UpdatedBy, productID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductSEONotFound
	}

	return r.FindByProductID(productID)
}

func (r *ProductSEORepository) SoftDelete(productID int64) error {
	query := "UPDATE ecom_product_seo SET deleted_at = NOW() WHERE product_id = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, productID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductSEONotFound
	}

	return nil
}

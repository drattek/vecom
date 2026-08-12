package mysql

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductDimensionsNotFound = errors.New("product dimensions not found")
)

type ProductDimensionsDTO struct {
	ProductID int64     `json:"productId"`
	Weight    string    `json:"weight"`
	Length    string    `json:"length"`
	Width     string    `json:"width"`
	Height    string    `json:"height"`
	Diameter  string    `json:"diameter"`
	Volume    string    `json:"volume"`
	CreatedBy int64     `json:"createdBy"`
	UpdatedBy *int64    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateProductDimensionsInput struct {
	ProductID int64
	Weight    string
	Length    string
	Width     string
	Height    string
	Diameter  string
	Volume    string
	CreatedBy int64
}

type UpdateProductDimensionsInput struct {
	Weight    string
	Length    string
	Width     string
	Height    string
	Diameter  string
	Volume    string
	UpdatedBy int64
}

type ProductDimensionsRepository struct {
	db *sql.DB
}

func NewProductDimensionsRepository(db *sql.DB) *ProductDimensionsRepository {
	return &ProductDimensionsRepository{db: db}
}

func (r *ProductDimensionsRepository) FindByProductID(productID int64) (*ProductDimensionsDTO, error) {
	query := `
		SELECT product_id, weight, length, width, height, diameter, volume, created_by, updated_by, created_at, updated_at
		FROM ecom_product_dimensions
		WHERE product_id = ? AND deleted_at IS NULL
	`

	var p ProductDimensionsDTO
	if err := r.db.QueryRow(query, productID).Scan(&p.ProductID, &p.Weight, &p.Length, &p.Width, &p.Height, &p.Diameter, &p.Volume, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductDimensionsNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductDimensionsRepository) Create(input CreateProductDimensionsInput) (*ProductDimensionsDTO, error) {
	query := `
		INSERT INTO ecom_product_dimensions (product_id, weight, length, width, height, diameter, volume, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, input.ProductID, input.Weight, input.Length, input.Width, input.Height, input.Diameter, input.Volume, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	return r.FindByProductID(input.ProductID)
}

func (r *ProductDimensionsRepository) Update(productID int64, input UpdateProductDimensionsInput) (*ProductDimensionsDTO, error) {
	query := `
		UPDATE ecom_product_dimensions
		SET weight = ?, length = ?, width = ?, height = ?, diameter = ?, volume = ?, updated_by = ?, updated_at = NOW()
		WHERE product_id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Weight, input.Length, input.Width, input.Height, input.Diameter, input.Volume, input.UpdatedBy, productID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductDimensionsNotFound
	}

	return r.FindByProductID(productID)
}

func (r *ProductDimensionsRepository) SoftDelete(productID int64) error {
	query := "UPDATE ecom_product_dimensions SET deleted_at = NOW() WHERE product_id = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, productID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductDimensionsNotFound
	}

	return nil
}

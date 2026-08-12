package mysql

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductPartNumberNotFound = errors.New("product part number not found")
)

type ProductPartNumberDTO struct {
	ProductID  int64     `json:"productId"`
	PartNumber string    `json:"partNumber"`
	Type       string    `json:"type"`
	BrandID    int64     `json:"brandId"`
	CreatedBy  int64     `json:"createdBy"`
	UpdatedBy  *int64    `json:"updatedBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type PaginatedProductPartNumbers struct {
	Data  []ProductPartNumberDTO `json:"data"`
	Total int                    `json:"total"`
}

type CreateProductPartNumberInput struct {
	ProductID  int64
	PartNumber string
	Type       string
	BrandID    int64
	CreatedBy  int64
}

type UpdateProductPartNumberInput struct {
	Type      string
	BrandID   int64
	UpdatedBy int64
}

type ProductPartNumbersRepository struct {
	db *sql.DB
}

func NewProductPartNumbersRepository(db *sql.DB) *ProductPartNumbersRepository {
	return &ProductPartNumbersRepository{db: db}
}

func (r *ProductPartNumbersRepository) FindPaginated(offset, pageSize int) (*PaginatedProductPartNumbers, error) {
	query := `
		SELECT product_id, part_number, type, brand_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_part_numbers
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partNumbers []ProductPartNumberDTO
	for rows.Next() {
		var p ProductPartNumberDTO
		if err := rows.Scan(&p.ProductID, &p.PartNumber, &p.Type, &p.BrandID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		partNumbers = append(partNumbers, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_part_numbers WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductPartNumbers{Data: partNumbers, Total: total}, nil
}

func (r *ProductPartNumbersRepository) FindByProductID(productID int64, offset, pageSize int) (*PaginatedProductPartNumbers, error) {
	query := `
		SELECT product_id, part_number, type, brand_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_part_numbers
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, productID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partNumbers []ProductPartNumberDTO
	for rows.Next() {
		var p ProductPartNumberDTO
		if err := rows.Scan(&p.ProductID, &p.PartNumber, &p.Type, &p.BrandID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		partNumbers = append(partNumbers, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_part_numbers WHERE product_id = ? AND deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery, productID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductPartNumbers{Data: partNumbers, Total: total}, nil
}

func (r *ProductPartNumbersRepository) Create(input CreateProductPartNumberInput) (*ProductPartNumberDTO, error) {
	query := `
		INSERT INTO ecom_product_part_numbers (product_id, part_number, type, brand_id, created_by)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, input.ProductID, input.PartNumber, input.Type, input.BrandID, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	// Retrieve the created record
	getQuery := `
		SELECT product_id, part_number, type, brand_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_part_numbers
		WHERE product_id = ? AND part_number = ? AND deleted_at IS NULL
	`

	var p ProductPartNumberDTO
	if err := r.db.QueryRow(getQuery, input.ProductID, input.PartNumber).Scan(&p.ProductID, &p.PartNumber, &p.Type, &p.BrandID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *ProductPartNumbersRepository) Update(productID int64, partNumber string, input UpdateProductPartNumberInput) (*ProductPartNumberDTO, error) {
	query := `
		UPDATE ecom_product_part_numbers
		SET type = ?, brand_id = ?, updated_by = ?, updated_at = NOW()
		WHERE product_id = ? AND part_number = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Type, input.BrandID, input.UpdatedBy, productID, partNumber)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductPartNumberNotFound
	}

	getQuery := `
		SELECT product_id, part_number, type, brand_id, created_by, updated_by, created_at, updated_at
		FROM ecom_product_part_numbers
		WHERE product_id = ? AND part_number = ? AND deleted_at IS NULL
	`

	var p ProductPartNumberDTO
	if err := r.db.QueryRow(getQuery, productID, partNumber).Scan(&p.ProductID, &p.PartNumber, &p.Type, &p.BrandID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *ProductPartNumbersRepository) SoftDelete(productID int64, partNumber string) error {
	query := "UPDATE ecom_product_part_numbers SET deleted_at = NOW() WHERE product_id = ? AND part_number = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, productID, partNumber)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductPartNumberNotFound
	}

	return nil
}

package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProductNotFound = errors.New("product not found")

type ProductDTO struct {
	ID               int64      `json:"id"`
	SKU              string     `json:"sku"`
	PartNumber       string     `json:"partNumber"`
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`
	ShortDescription *string    `json:"shortDescription,omitempty"`
	BrandID          *int64     `json:"brandId,omitempty"`
	CategoryID       *int64     `json:"categoryId,omitempty"`
	ProductType      string     `json:"productType"`
	Status           *string    `json:"status,omitempty"`
	IsSellable       bool       `json:"isSellable"`
	IsStockable      bool       `json:"isStockable"`
	SourceID         int64      `json:"sourceId"`
	CreatedBy        int64      `json:"createdBy"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedProducts struct {
	Total    int64        `json:"total"`
	Offset   int          `json:"offset"`
	PageSize int          `json:"pageSize"`
	Products []ProductDTO `json:"products"`
}

type CreateProductInput struct {
	SKU              string
	PartNumber       string
	Name             string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
	ProductType      string
	Status           *string
	IsSellable       bool
	IsStockable      bool
	SourceID         int64
	CreatedBy        int64
}

type UpdateProductInput struct {
	SKU              string
	PartNumber       string
	Name             string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
	ProductType      string
	Status           *string
	IsSellable       bool
	IsStockable      bool
	SourceID         int64
	UpdatedBy        int64
}

type ProductRepository struct {
	db Querier
}

func NewProductRepository(db Querier) *ProductRepository {
	return &ProductRepository{db: db}
}

const productColumns = `
	id, sku, part_number, name, description, short_description, brand_id, category_id,
	product_type, status, is_sellable, is_stockable, source_id,
	created_by, updated_by, created_at, updated_at, deleted_at
`

func (r *ProductRepository) FindPaginated(offset, pageSize int) (*PaginatedProducts, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_products WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting products: %w", err)
	}

	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying products: %w", err)
	}
	defer rows.Close()

	products := make([]ProductDTO, 0)
	for rows.Next() {
		product, scanErr := scanProduct(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		products = append(products, product)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return &PaginatedProducts{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Products: products,
	}, nil
}

func (r *ProductRepository) FindByID(id int64) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanProductRow(row)
}

func (r *ProductRepository) FindBySKU(sku string) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE sku = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, sku)
	return scanProductRow(row)
}

// FindBySourceAndPartNumber busca por part_number dentro de una fuente, no por sku — sku y
// part_number pueden ser distintos fuera del flujo de Nissan (donde hoy son iguales, ver
// upsertNissanProduct en sync_nissan.go), y la resolución de ecom_part_number_supersessions
// está definida en términos de part_number, no de sku. Se apoya en el unique key
// (source_id, part_number) de ecom_products para garantizar como mucho un resultado.
func (r *ProductRepository) FindBySourceAndPartNumber(sourceID int64, partNumber string) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE source_id = ? AND part_number = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, sourceID, partNumber)
	return scanProductRow(row)
}

func (r *ProductRepository) Create(input CreateProductInput) (*ProductDTO, error) {
	query := `
		INSERT INTO ecom_products (
			sku, part_number, name, description, short_description, brand_id, category_id,
			product_type, status, is_sellable, is_stockable, source_id, created_by, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(
		query,
		input.SKU, input.PartNumber, input.Name, input.Description, input.ShortDescription,
		input.BrandID, input.CategoryID, input.ProductType, input.Status,
		input.IsSellable, input.IsStockable, input.SourceID, input.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating product: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *ProductRepository) Update(id int64, input UpdateProductInput) (*ProductDTO, error) {
	query := `
		UPDATE ecom_products
		SET sku = ?, part_number = ?, name = ?, description = ?, short_description = ?,
			brand_id = ?, category_id = ?, product_type = ?, status = ?,
			is_sellable = ?, is_stockable = ?, source_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(
		query,
		input.SKU, input.PartNumber, input.Name, input.Description, input.ShortDescription,
		input.BrandID, input.CategoryID, input.ProductType, input.Status,
		input.IsSellable, input.IsStockable, input.SourceID, input.UpdatedBy, id,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating product: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrProductNotFound
	}

	return r.FindByID(id)
}

// UpdateCategoryID sets category_id alone, leaving every other column
// untouched — used when a marketplace sync assigns or replaces a product's
// category from the channel's own taxonomy (see
// sync.MercadoLibreProductSyncService.syncCategoryMapping) instead of a full
// product edit.
func (r *ProductRepository) UpdateCategoryID(id, categoryID, updatedBy int64) error {
	query := `
		UPDATE ecom_products
		SET category_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, categoryID, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating product category: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *ProductRepository) SoftDelete(id int64) error {
	query := `
		UPDATE ecom_products
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting product: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func scanProduct(rows *sql.Rows) (ProductDTO, error) {
	var p ProductDTO
	err := rows.Scan(
		&p.ID, &p.SKU, &p.PartNumber, &p.Name, &p.Description, &p.ShortDescription,
		&p.BrandID, &p.CategoryID, &p.ProductType, &p.Status, &p.IsSellable, &p.IsStockable,
		&p.SourceID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		return ProductDTO{}, fmt.Errorf("error scanning product: %w", err)
	}
	return p, nil
}

func scanProductRow(row *sql.Row) (*ProductDTO, error) {
	var p ProductDTO
	err := row.Scan(
		&p.ID, &p.SKU, &p.PartNumber, &p.Name, &p.Description, &p.ShortDescription,
		&p.BrandID, &p.CategoryID, &p.ProductType, &p.Status, &p.IsSellable, &p.IsStockable,
		&p.SourceID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error scanning product: %w", err)
	}
	return &p, nil
}

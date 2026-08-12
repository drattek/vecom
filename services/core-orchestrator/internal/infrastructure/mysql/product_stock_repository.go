package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProductStockNotFound = errors.New("product stock not found")

type ProductStockDTO struct {
	ID           int64      `json:"id"`
	ProductID    int64      `json:"productId"`
	BranchID     int64      `json:"branchId"`
	WarehouseID  int64      `json:"warehouseId"`
	AvailableQty int        `json:"availableQty"`
	LastSyncAt   *time.Time `json:"lastSyncAt,omitempty"`
	UpdatedBy    *int64     `json:"updatedBy,omitempty"`
}

type PaginatedProductStock struct {
	Total    int64             `json:"total"`
	Offset   int               `json:"offset"`
	PageSize int               `json:"pageSize"`
	Stock    []ProductStockDTO `json:"stock"`
}

type CreateProductStockInput struct {
	ProductID    int64
	BranchID     int64
	WarehouseID  int64
	AvailableQty int
}

type UpdateProductStockInput struct {
	AvailableQty int
	UpdatedBy    int64
}

type ProductStockRepository struct {
	db Querier
}

func NewProductStockRepository(db Querier) *ProductStockRepository {
	return &ProductStockRepository{db: db}
}

func (r *ProductStockRepository) FindPaginated(offset, pageSize int) (*PaginatedProductStock, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_product_stock").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting product stock: %w", err)
	}

	query := `
		SELECT id, product_id, branch_id, warehouse_id, available_qty, last_sync_at, updated_by
		FROM ecom_product_stock
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying product stock: %w", err)
	}
	defer rows.Close()

	stock := make([]ProductStockDTO, 0)
	for rows.Next() {
		ps, scanErr := scanProductStock(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		stock = append(stock, ps)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating product stock: %w", err)
	}

	return &PaginatedProductStock{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Stock:    stock,
	}, nil
}

func (r *ProductStockRepository) FindByID(id int64) (*ProductStockDTO, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, available_qty, last_sync_at, updated_by
		FROM ecom_product_stock
		WHERE id = ?
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanProductStockRow(row)
}

func (r *ProductStockRepository) FindByProductBranchWarehouse(productID, branchID, warehouseID int64) (*ProductStockDTO, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, available_qty, last_sync_at, updated_by
		FROM ecom_product_stock
		WHERE product_id = ? AND branch_id = ? AND warehouse_id = ?
		LIMIT 1
	`

	row := r.db.QueryRow(query, productID, branchID, warehouseID)
	return scanProductStockRow(row)
}

func (r *ProductStockRepository) FindByProductID(productID int64) ([]ProductStockDTO, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, available_qty, last_sync_at, updated_by
		FROM ecom_product_stock
		WHERE product_id = ?
		ORDER BY branch_id ASC, warehouse_id ASC
	`

	rows, err := r.db.Query(query, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product stock by product: %w", err)
	}
	defer rows.Close()

	stock := make([]ProductStockDTO, 0)
	for rows.Next() {
		ps, scanErr := scanProductStock(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		stock = append(stock, ps)
	}

	return stock, rows.Err()
}

func (r *ProductStockRepository) Create(input CreateProductStockInput) (*ProductStockDTO, error) {
	query := `
		INSERT INTO ecom_product_stock (product_id, branch_id, warehouse_id, available_qty)
		VALUES (?, ?, ?, ?)
	`

	result, err := r.db.Exec(query, input.ProductID, input.BranchID, input.WarehouseID, input.AvailableQty)
	if err != nil {
		return nil, fmt.Errorf("error creating product stock: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *ProductStockRepository) Update(id int64, input UpdateProductStockInput) (*ProductStockDTO, error) {
	query := `
		UPDATE ecom_product_stock
		SET available_qty = ?, updated_by = ?, last_sync_at = NOW()
		WHERE id = ?
	`

	result, err := r.db.Exec(query, input.AvailableQty, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating product stock: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrProductStockNotFound
	}

	return r.FindByID(id)
}

func scanProductStock(rows *sql.Rows) (ProductStockDTO, error) {
	var ps ProductStockDTO
	err := rows.Scan(
		&ps.ID,
		&ps.ProductID,
		&ps.BranchID,
		&ps.WarehouseID,
		&ps.AvailableQty,
		&ps.LastSyncAt,
		&ps.UpdatedBy,
	)
	if err != nil {
		return ProductStockDTO{}, fmt.Errorf("error scanning product stock: %w", err)
	}
	return ps, nil
}

func scanProductStockRow(row *sql.Row) (*ProductStockDTO, error) {
	var ps ProductStockDTO
	err := row.Scan(
		&ps.ID,
		&ps.ProductID,
		&ps.BranchID,
		&ps.WarehouseID,
		&ps.AvailableQty,
		&ps.LastSyncAt,
		&ps.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductStockNotFound
		}
		return nil, fmt.Errorf("error scanning product stock: %w", err)
	}
	return &ps, nil
}

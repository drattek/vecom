package mysql

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrStockMovementNotFound = errors.New("stock movement not found")
)

type StockMovementDTO struct {
	ID             int64     `json:"id"`
	ProductID      int64     `json:"productId"`
	BranchID       int64     `json:"branchId"`
	WarehouseID    int64     `json:"warehouseId"`
	MovementType   string    `json:"movementType"`
	QuantityBefore int       `json:"quantityBefore"`
	QuantityChange int       `json:"quantityChange"`
	QuantityAfter  int       `json:"quantityAfter"`
	UpdatedBy      int64     `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type PaginatedStockMovements struct {
	Data  []StockMovementDTO `json:"data"`
	Total int                `json:"total"`
}

type CreateStockMovementInput struct {
	ProductID      int64
	BranchID       int64
	WarehouseID    int64
	MovementType   string
	QuantityBefore int
	QuantityChange int
	QuantityAfter  int
	UpdatedBy      int64
}

type StockMovementsRepository struct {
	db Querier
}

func NewStockMovementsRepository(db Querier) *StockMovementsRepository {
	return &StockMovementsRepository{db: db}
}

func (r *StockMovementsRepository) FindPaginated(offset, pageSize int) (*PaginatedStockMovements, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, movement_type, 
		       quantity_before, quantity_change, quantity_after, updated_by, created_at, updated_at
		FROM ecom_stock_movements
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []StockMovementDTO
	for rows.Next() {
		var s StockMovementDTO
		if err := rows.Scan(&s.ID, &s.ProductID, &s.BranchID, &s.WarehouseID, &s.MovementType,
			&s.QuantityBefore, &s.QuantityChange, &s.QuantityAfter, &s.UpdatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, s)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_stock_movements WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedStockMovements{Data: movements, Total: total}, nil
}

func (r *StockMovementsRepository) FindByID(id int64) (*StockMovementDTO, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, movement_type, 
		       quantity_before, quantity_change, quantity_after, updated_by, created_at, updated_at
		FROM ecom_stock_movements
		WHERE id = ? AND deleted_at IS NULL
	`

	var s StockMovementDTO
	if err := r.db.QueryRow(query, id).Scan(&s.ID, &s.ProductID, &s.BranchID, &s.WarehouseID,
		&s.MovementType, &s.QuantityBefore, &s.QuantityChange, &s.QuantityAfter, &s.UpdatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrStockMovementNotFound
		}
		return nil, err
	}

	return &s, nil
}

func (r *StockMovementsRepository) FindByProductID(productID int64, offset, pageSize int) (*PaginatedStockMovements, error) {
	query := `
		SELECT id, product_id, branch_id, warehouse_id, movement_type, 
		       quantity_before, quantity_change, quantity_after, updated_by, created_at, updated_at
		FROM ecom_stock_movements
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, productID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []StockMovementDTO
	for rows.Next() {
		var s StockMovementDTO
		if err := rows.Scan(&s.ID, &s.ProductID, &s.BranchID, &s.WarehouseID, &s.MovementType,
			&s.QuantityBefore, &s.QuantityChange, &s.QuantityAfter, &s.UpdatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, s)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_stock_movements WHERE product_id = ? AND deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery, productID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedStockMovements{Data: movements, Total: total}, nil
}

func (r *StockMovementsRepository) Create(input CreateStockMovementInput) (*StockMovementDTO, error) {
	query := `
		INSERT INTO ecom_stock_movements (product_id, branch_id, warehouse_id, movement_type, 
		                                   quantity_before, quantity_change, quantity_after, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query, input.ProductID, input.BranchID, input.WarehouseID, input.MovementType,
		input.QuantityBefore, input.QuantityChange, input.QuantityAfter, input.UpdatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(id)
}

func (r *StockMovementsRepository) SoftDelete(id int64) error {
	query := "UPDATE ecom_stock_movements SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrStockMovementNotFound
	}

	return nil
}

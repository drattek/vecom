package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrWarehouseNotFound = errors.New("warehouse not found")

type WarehouseDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	BranchID  int64      `json:"branchId"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedWarehouses struct {
	Total      int64          `json:"total"`
	Offset     int            `json:"offset"`
	PageSize   int            `json:"pageSize"`
	Warehouses []WarehouseDTO `json:"warehouses"`
}

type CreateWarehouseInput struct {
	Name      string
	BranchID  int64
	CreatedBy int64
}

type UpdateWarehouseInput struct {
	Name      string
	BranchID  int64
	UpdatedBy int64
}

type WarehousesRepository struct {
	db Querier
}

func NewWarehousesRepository(db Querier) *WarehousesRepository {
	return &WarehousesRepository{db: db}
}

func (r *WarehousesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedWarehouses, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_warehouses WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting warehouses: %w", err)
	}

	query := `
		SELECT id, name, branch_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_warehouses
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying warehouses: %w", err)
	}
	defer rows.Close()

	warehouses := make([]WarehouseDTO, 0)
	for rows.Next() {
		warehouse, scanErr := scanWarehouse(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		warehouses = append(warehouses, warehouse)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating warehouses: %w", err)
	}

	return &PaginatedWarehouses{
		Total:      total,
		Offset:     offset,
		PageSize:   pageSize,
		Warehouses: warehouses,
	}, nil
}

func (r *WarehousesRepository) FindByID(ctx context.Context, id int64) (*WarehouseDTO, error) {
	query := `
		SELECT id, name, branch_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_warehouses
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanWarehouseRow(row)
}

func (r *WarehousesRepository) FindByBranchID(ctx context.Context, branchID int64) ([]WarehouseDTO, error) {
	query := `
		SELECT id, name, branch_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_warehouses
		WHERE branch_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, branchID)
	if err != nil {
		return nil, fmt.Errorf("error querying warehouses by branch: %w", err)
	}
	defer rows.Close()

	warehouses := make([]WarehouseDTO, 0)
	for rows.Next() {
		warehouse, scanErr := scanWarehouse(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		warehouses = append(warehouses, warehouse)
	}

	return warehouses, rows.Err()
}

func (r *WarehousesRepository) FindByNameAndBranch(ctx context.Context, name string, branchID int64) (*WarehouseDTO, error) {
	query := `
		SELECT id, name, branch_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_warehouses
		WHERE name = ? AND branch_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, name, branchID)
	return scanWarehouseRow(row)
}

// FindNamedBranchIDsByWarehouseName devuelve las sucursales activas con nombre
// no vacío que ya tienen un almacén activo llamado name. Lo usa el sync de
// stock del ERP para inferir la sucursal cuando el ERP no reporta su nombre.
func (r *WarehousesRepository) FindNamedBranchIDsByWarehouseName(ctx context.Context, name string) ([]int64, error) {
	query := `
		SELECT DISTINCT w.branch_id
		FROM ecom_warehouses w
		JOIN ecom_branches b ON b.id = w.branch_id AND b.deleted_at IS NULL
		WHERE w.name = ? AND w.deleted_at IS NULL AND b.name REGEXP '[[:alnum:]]'
	`

	rows, err := r.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("error finding branches by warehouse name: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// ensureNamedBranch rechaza crear o mover un almacén bajo una sucursal inexistente, dada de
// baja o sin nombre (la causa de los almacenes colgados de "Sin sucursal").
func (r *WarehousesRepository) ensureNamedBranch(ctx context.Context, branchID int64) error {
	var name string
	err := r.db.QueryRowContext(ctx, "SELECT name FROM ecom_branches WHERE id = ? AND deleted_at IS NULL", branchID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBranchNotFound
	}
	if err != nil {
		return fmt.Errorf("error checking warehouse branch: %w", err)
	}
	if !HasVisibleName(name) {
		return ErrInvalidLocationName
	}
	return nil
}

func (r *WarehousesRepository) Create(ctx context.Context, input CreateWarehouseInput) (*WarehouseDTO, error) {
	if !HasVisibleName(input.Name) {
		return nil, ErrInvalidLocationName
	}
	if err := r.ensureNamedBranch(ctx, input.BranchID); err != nil {
		return nil, err
	}

	query := `
		INSERT INTO ecom_warehouses (name, branch_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.BranchID, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating warehouse: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *WarehousesRepository) Update(ctx context.Context, id int64, input UpdateWarehouseInput) (*WarehouseDTO, error) {
	if !HasVisibleName(input.Name) {
		return nil, ErrInvalidLocationName
	}
	if err := r.ensureNamedBranch(ctx, input.BranchID); err != nil {
		return nil, err
	}

	query := `
		UPDATE ecom_warehouses
		SET name = ?, branch_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.BranchID, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating warehouse: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrWarehouseNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *WarehousesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_warehouses
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting warehouse: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrWarehouseNotFound
	}

	return nil
}

func scanWarehouse(rows *sql.Rows) (WarehouseDTO, error) {
	var warehouse WarehouseDTO
	err := rows.Scan(
		&warehouse.ID,
		&warehouse.Name,
		&warehouse.BranchID,
		&warehouse.CreatedBy,
		&warehouse.UpdatedBy,
		&warehouse.CreatedAt,
		&warehouse.UpdatedAt,
		&warehouse.DeletedAt,
	)
	if err != nil {
		return WarehouseDTO{}, fmt.Errorf("error scanning warehouse: %w", err)
	}
	return warehouse, nil
}

func scanWarehouseRow(row *sql.Row) (*WarehouseDTO, error) {
	var warehouse WarehouseDTO
	err := row.Scan(
		&warehouse.ID,
		&warehouse.Name,
		&warehouse.BranchID,
		&warehouse.CreatedBy,
		&warehouse.UpdatedBy,
		&warehouse.CreatedAt,
		&warehouse.UpdatedAt,
		&warehouse.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWarehouseNotFound
		}
		return nil, fmt.Errorf("error scanning warehouse: %w", err)
	}
	return &warehouse, nil
}

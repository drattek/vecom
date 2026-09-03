package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProductEquipmentCompatibilityNotFound = errors.New("product equipment compatibility not found")
var ErrProductEquipmentCompatibilityAlreadyExists = errors.New("product equipment compatibility already exists")
var ErrProductEquipmentCompatibilityInvalidReference = errors.New("product equipment compatibility invalid reference")

type ProductEquipmentCompatibilityDTO struct {
	ID                 int64      `json:"id"`
	ProductID          int64      `json:"productId"`
	EquipmentFitmentID int64      `json:"equipmentFitmentId"`
	CreatedBy          int64      `json:"createdBy"`
	UpdatedBy          *int64     `json:"updatedBy,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedProductEquipmentCompatibilities struct {
	Total           int64                              `json:"total"`
	Offset          int                                `json:"offset"`
	PageSize        int                                `json:"pageSize"`
	Compatibilities []ProductEquipmentCompatibilityDTO `json:"compatibilities"`
}

type CreateProductEquipmentCompatibilityInput struct {
	ProductID          int64
	EquipmentFitmentID int64
	CreatedBy          int64
}

type ProductEquipmentCompatibilityRepository struct {
	db Querier
}

func NewProductEquipmentCompatibilityRepository(db Querier) *ProductEquipmentCompatibilityRepository {
	return &ProductEquipmentCompatibilityRepository{db: db}
}

func (r *ProductEquipmentCompatibilityRepository) FindByProductID(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedProductEquipmentCompatibilities, error) {
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ecom_product_equipment_compatibility WHERE product_id = ? AND deleted_at IS NULL",
		productID,
	).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting product equipment compatibilities: %w", err)
	}

	query := `
		SELECT id, product_id, equipment_fitment_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_equipment_compatibility
		WHERE product_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, productID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying product equipment compatibilities: %w", err)
	}
	defer rows.Close()

	compatibilities := make([]ProductEquipmentCompatibilityDTO, 0)
	for rows.Next() {
		compatibility, scanErr := scanProductEquipmentCompatibility(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		compatibilities = append(compatibilities, compatibility)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating product equipment compatibilities: %w", err)
	}

	return &PaginatedProductEquipmentCompatibilities{
		Total:           total,
		Offset:          offset,
		PageSize:        pageSize,
		Compatibilities: compatibilities,
	}, nil
}

func (r *ProductEquipmentCompatibilityRepository) FindByID(ctx context.Context, id int64) (*ProductEquipmentCompatibilityDTO, error) {
	query := `
		SELECT id, product_id, equipment_fitment_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_equipment_compatibility
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanProductEquipmentCompatibilityRow(row)
}

func (r *ProductEquipmentCompatibilityRepository) Create(ctx context.Context, input CreateProductEquipmentCompatibilityInput) (*ProductEquipmentCompatibilityDTO, error) {
	query := `
		INSERT INTO ecom_product_equipment_compatibility (product_id, equipment_fitment_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.EquipmentFitmentID, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrProductEquipmentCompatibilityAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrProductEquipmentCompatibilityInvalidReference
		}
		return nil, fmt.Errorf("error creating product equipment compatibility: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *ProductEquipmentCompatibilityRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_product_equipment_compatibility
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting product equipment compatibility: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductEquipmentCompatibilityNotFound
	}

	return nil
}

func scanProductEquipmentCompatibility(rows *sql.Rows) (ProductEquipmentCompatibilityDTO, error) {
	var compatibility ProductEquipmentCompatibilityDTO
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&compatibility.ID,
		&compatibility.ProductID,
		&compatibility.EquipmentFitmentID,
		&compatibility.CreatedBy,
		&updatedBy,
		&compatibility.CreatedAt,
		&compatibility.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return ProductEquipmentCompatibilityDTO{}, fmt.Errorf("error scanning product equipment compatibility: %w", err)
	}

	if updatedBy.Valid {
		compatibility.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		compatibility.DeletedAt = &deletedAt.Time
	}

	return compatibility, nil
}

func scanProductEquipmentCompatibilityRow(row *sql.Row) (*ProductEquipmentCompatibilityDTO, error) {
	var compatibility ProductEquipmentCompatibilityDTO
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&compatibility.ID,
		&compatibility.ProductID,
		&compatibility.EquipmentFitmentID,
		&compatibility.CreatedBy,
		&updatedBy,
		&compatibility.CreatedAt,
		&compatibility.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductEquipmentCompatibilityNotFound
		}
		return nil, fmt.Errorf("error scanning product equipment compatibility: %w", err)
	}

	if updatedBy.Valid {
		compatibility.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		compatibility.DeletedAt = &deletedAt.Time
	}

	return &compatibility, nil
}

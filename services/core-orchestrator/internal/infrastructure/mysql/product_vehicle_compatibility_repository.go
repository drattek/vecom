package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProductVehicleCompatibilityNotFound = errors.New("product vehicle compatibility not found")
var ErrProductVehicleCompatibilityAlreadyExists = errors.New("product vehicle compatibility already exists")
var ErrProductVehicleCompatibilityInvalidReference = errors.New("product vehicle compatibility invalid reference")

type ProductVehicleCompatibilityDTO struct {
	ID               int64      `json:"id"`
	ProductID        int64      `json:"productId"`
	VehicleFitmentID int64      `json:"vehicleFitmentId"`
	Motor            string     `json:"motor"`
	Position         string     `json:"position"`
	Side             string     `json:"side"`
	CreatedBy        int64      `json:"createdBy"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedProductVehicleCompatibilities struct {
	Total           int64                            `json:"total"`
	Offset          int                              `json:"offset"`
	PageSize        int                              `json:"pageSize"`
	Compatibilities []ProductVehicleCompatibilityDTO `json:"compatibilities"`
}

type CreateProductVehicleCompatibilityInput struct {
	ProductID        int64
	VehicleFitmentID int64
	Motor            string
	Position         string
	Side             string
	CreatedBy        int64
}

type ProductVehicleCompatibilityRepository struct {
	db *sql.DB
}

func NewProductVehicleCompatibilityRepository(db *sql.DB) *ProductVehicleCompatibilityRepository {
	return &ProductVehicleCompatibilityRepository{db: db}
}

func (r *ProductVehicleCompatibilityRepository) FindByProductID(productID int64, offset, pageSize int) (*PaginatedProductVehicleCompatibilities, error) {
	var total int64
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM ecom_product_vehicle_compatibility WHERE product_id = ? AND deleted_at IS NULL",
		productID,
	).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting product vehicle compatibilities: %w", err)
	}

	query := `
		SELECT id, product_id, vehicle_fitment_id, motor, position, side, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_vehicle_compatibility
		WHERE product_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, productID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying product vehicle compatibilities: %w", err)
	}
	defer rows.Close()

	compatibilities := make([]ProductVehicleCompatibilityDTO, 0)
	for rows.Next() {
		compatibility, scanErr := scanProductVehicleCompatibility(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		compatibilities = append(compatibilities, compatibility)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating product vehicle compatibilities: %w", err)
	}

	return &PaginatedProductVehicleCompatibilities{
		Total:           total,
		Offset:          offset,
		PageSize:        pageSize,
		Compatibilities: compatibilities,
	}, nil
}

func (r *ProductVehicleCompatibilityRepository) FindByID(id int64) (*ProductVehicleCompatibilityDTO, error) {
	query := `
		SELECT id, product_id, vehicle_fitment_id, motor, position, side, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_vehicle_compatibility
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanProductVehicleCompatibilityRow(row)
}

func (r *ProductVehicleCompatibilityRepository) Create(input CreateProductVehicleCompatibilityInput) (*ProductVehicleCompatibilityDTO, error) {
	query := `
		INSERT INTO ecom_product_vehicle_compatibility (product_id, vehicle_fitment_id, motor, position, side, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.ProductID, input.VehicleFitmentID, input.Motor, input.Position, input.Side, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrProductVehicleCompatibilityAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrProductVehicleCompatibilityInvalidReference
		}
		return nil, fmt.Errorf("error creating product vehicle compatibility: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *ProductVehicleCompatibilityRepository) SoftDelete(id int64) error {
	query := `
		UPDATE ecom_product_vehicle_compatibility
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting product vehicle compatibility: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductVehicleCompatibilityNotFound
	}

	return nil
}

func scanProductVehicleCompatibility(rows *sql.Rows) (ProductVehicleCompatibilityDTO, error) {
	var compatibility ProductVehicleCompatibilityDTO
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&compatibility.ID,
		&compatibility.ProductID,
		&compatibility.VehicleFitmentID,
		&compatibility.Motor,
		&compatibility.Position,
		&compatibility.Side,
		&compatibility.CreatedBy,
		&updatedBy,
		&compatibility.CreatedAt,
		&compatibility.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return ProductVehicleCompatibilityDTO{}, fmt.Errorf("error scanning product vehicle compatibility: %w", err)
	}

	if updatedBy.Valid {
		compatibility.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		compatibility.DeletedAt = &deletedAt.Time
	}

	return compatibility, nil
}

func scanProductVehicleCompatibilityRow(row *sql.Row) (*ProductVehicleCompatibilityDTO, error) {
	var compatibility ProductVehicleCompatibilityDTO
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&compatibility.ID,
		&compatibility.ProductID,
		&compatibility.VehicleFitmentID,
		&compatibility.Motor,
		&compatibility.Position,
		&compatibility.Side,
		&compatibility.CreatedBy,
		&updatedBy,
		&compatibility.CreatedAt,
		&compatibility.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductVehicleCompatibilityNotFound
		}
		return nil, fmt.Errorf("error scanning product vehicle compatibility: %w", err)
	}

	if updatedBy.Valid {
		compatibility.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		compatibility.DeletedAt = &deletedAt.Time
	}

	return &compatibility, nil
}

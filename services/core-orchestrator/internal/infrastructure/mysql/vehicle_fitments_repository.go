package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrVehicleFitmentNotFound = errors.New("vehicle fitment not found")
var ErrVehicleFitmentAlreadyExists = errors.New("vehicle fitment already exists")
var ErrVehicleFitmentInvalidReference = errors.New("vehicle fitment invalid reference")

type VehicleFitmentDTO struct {
	ID        int64      `json:"id"`
	BrandID   int64      `json:"brandId"`
	Model     string     `json:"model"`
	YearStart int        `json:"yearStart"`
	YearEnd   *int       `json:"yearEnd,omitempty"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedVehicleFitments struct {
	Total    int64               `json:"total"`
	Offset   int                 `json:"offset"`
	PageSize int                 `json:"pageSize"`
	Fitments []VehicleFitmentDTO `json:"fitments"`
}

type CreateVehicleFitmentInput struct {
	BrandID   int64
	Model     string
	YearStart int
	YearEnd   *int
	CreatedBy int64
}

type UpdateVehicleFitmentInput struct {
	BrandID   int64
	Model     string
	YearStart int
	YearEnd   *int
	UpdatedBy int64
}

type VehicleFitmentsRepository struct {
	db Querier
}

func NewVehicleFitmentsRepository(db Querier) *VehicleFitmentsRepository {
	return &VehicleFitmentsRepository{db: db}
}

func (r *VehicleFitmentsRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedVehicleFitments, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_vehicle_fitments WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting vehicle fitments: %w", err)
	}

	query := `
		SELECT id, brand_id, model, year_start, year_end, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_vehicle_fitments
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying vehicle fitments: %w", err)
	}
	defer rows.Close()

	fitments := make([]VehicleFitmentDTO, 0)
	for rows.Next() {
		fitment, scanErr := scanVehicleFitment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		fitments = append(fitments, fitment)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating vehicle fitments: %w", err)
	}

	return &PaginatedVehicleFitments{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Fitments: fitments,
	}, nil
}

func (r *VehicleFitmentsRepository) FindByID(ctx context.Context, id int64) (*VehicleFitmentDTO, error) {
	query := `
		SELECT id, brand_id, model, year_start, year_end, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_vehicle_fitments
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanVehicleFitmentRow(row)
}

func (r *VehicleFitmentsRepository) FindByUniqueKey(ctx context.Context, brandID int64, model string, yearStart int, yearEnd *int) (*VehicleFitmentDTO, error) {
	query := `
		SELECT id, brand_id, model, year_start, year_end, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_vehicle_fitments
		WHERE brand_id = ? AND model = ? AND year_start = ? AND year_end <=> ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, brandID, model, yearStart, yearEnd)
	return scanVehicleFitmentRow(row)
}

func (r *VehicleFitmentsRepository) Create(ctx context.Context, input CreateVehicleFitmentInput) (*VehicleFitmentDTO, error) {
	query := `
		INSERT INTO ecom_vehicle_fitments (brand_id, model, year_start, year_end, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.Model, input.YearStart, input.YearEnd, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrVehicleFitmentAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrVehicleFitmentInvalidReference
		}
		return nil, fmt.Errorf("error creating vehicle fitment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *VehicleFitmentsRepository) Update(ctx context.Context, id int64, input UpdateVehicleFitmentInput) (*VehicleFitmentDTO, error) {
	query := `
		UPDATE ecom_vehicle_fitments
		SET brand_id = ?, model = ?, year_start = ?, year_end = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.Model, input.YearStart, input.YearEnd, input.UpdatedBy, id)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrVehicleFitmentAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrVehicleFitmentInvalidReference
		}
		return nil, fmt.Errorf("error updating vehicle fitment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrVehicleFitmentNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *VehicleFitmentsRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_vehicle_fitments
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting vehicle fitment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrVehicleFitmentNotFound
	}

	return nil
}

func scanVehicleFitment(rows *sql.Rows) (VehicleFitmentDTO, error) {
	var fitment VehicleFitmentDTO
	var yearEnd sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.Model,
		&fitment.YearStart,
		&yearEnd,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return VehicleFitmentDTO{}, fmt.Errorf("error scanning vehicle fitment: %w", err)
	}

	if yearEnd.Valid {
		year := int(yearEnd.Int64)
		fitment.YearEnd = &year
	}
	if updatedBy.Valid {
		fitment.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		fitment.DeletedAt = &deletedAt.Time
	}

	return fitment, nil
}

func scanVehicleFitmentRow(row *sql.Row) (*VehicleFitmentDTO, error) {
	var fitment VehicleFitmentDTO
	var yearEnd sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.Model,
		&fitment.YearStart,
		&yearEnd,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVehicleFitmentNotFound
		}
		return nil, fmt.Errorf("error scanning vehicle fitment: %w", err)
	}

	if yearEnd.Valid {
		year := int(yearEnd.Int64)
		fitment.YearEnd = &year
	}
	if updatedBy.Valid {
		fitment.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		fitment.DeletedAt = &deletedAt.Time
	}

	return &fitment, nil
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrEquipmentFitmentNotFound = errors.New("equipment fitment not found")
var ErrEquipmentFitmentInvalidReference = errors.New("equipment fitment invalid reference")

type EquipmentFitmentDTO struct {
	ID              int64      `json:"id"`
	BrandID         int64      `json:"brandId"`
	EquipmentTypeID int64      `json:"equipmentTypeId"`
	Model           *string    `json:"model,omitempty"`
	Serie           *string    `json:"serie,omitempty"`
	CreatedBy       int64      `json:"createdBy"`
	UpdatedBy       *int64     `json:"updatedBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedEquipmentFitments struct {
	Total    int                   `json:"total"`
	Offset   int                   `json:"offset"`
	PageSize int                   `json:"pageSize"`
	Fitments []EquipmentFitmentDTO `json:"fitments"`
}

type CreateEquipmentFitmentInput struct {
	BrandID         int64
	EquipmentTypeID int64
	Model           *string
	Serie           *string
	CreatedBy       int64
}

type UpdateEquipmentFitmentInput struct {
	BrandID         int64
	EquipmentTypeID int64
	Model           *string
	Serie           *string
	UpdatedBy       int64
}

type EquipmentFitmentsRepository struct {
	db Querier
}

func NewEquipmentFitmentsRepository(db Querier) *EquipmentFitmentsRepository {
	return &EquipmentFitmentsRepository{db: db}
}

func (r *EquipmentFitmentsRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedEquipmentFitments, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_equipment_fitments WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting equipment fitments: %w", err)
	}

	query := `
		SELECT id, brand_id, equipment_type_id, model, serie, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_equipment_fitments
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying equipment fitments: %w", err)
	}
	defer rows.Close()

	fitments := make([]EquipmentFitmentDTO, 0)
	for rows.Next() {
		fitment, scanErr := scanEquipmentFitment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		fitments = append(fitments, fitment)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating equipment fitments: %w", err)
	}

	return &PaginatedEquipmentFitments{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Fitments: fitments,
	}, nil
}

func (r *EquipmentFitmentsRepository) FindByID(ctx context.Context, id int64) (*EquipmentFitmentDTO, error) {
	query := `
		SELECT id, brand_id, equipment_type_id, model, serie, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_equipment_fitments
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanEquipmentFitmentRow(row)
}

func (r *EquipmentFitmentsRepository) Create(ctx context.Context, input CreateEquipmentFitmentInput) (*EquipmentFitmentDTO, error) {
	query := `
		INSERT INTO ecom_equipment_fitments (brand_id, equipment_type_id, model, serie, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.EquipmentTypeID, input.Model, input.Serie, input.CreatedBy)
	if err != nil {
		if isForeignKeyConstraintError(err) {
			return nil, ErrEquipmentFitmentInvalidReference
		}
		return nil, fmt.Errorf("error creating equipment fitment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *EquipmentFitmentsRepository) Update(ctx context.Context, id int64, input UpdateEquipmentFitmentInput) (*EquipmentFitmentDTO, error) {
	query := `
		UPDATE ecom_equipment_fitments
		SET brand_id = ?, equipment_type_id = ?, model = ?, serie = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.EquipmentTypeID, input.Model, input.Serie, input.UpdatedBy, id)
	if err != nil {
		if isForeignKeyConstraintError(err) {
			return nil, ErrEquipmentFitmentInvalidReference
		}
		return nil, fmt.Errorf("error updating equipment fitment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrEquipmentFitmentNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *EquipmentFitmentsRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_equipment_fitments
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting equipment fitment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrEquipmentFitmentNotFound
	}

	return nil
}

func scanEquipmentFitment(rows *sql.Rows) (EquipmentFitmentDTO, error) {
	var fitment EquipmentFitmentDTO
	var model sql.NullString
	var serie sql.NullString
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.EquipmentTypeID,
		&model,
		&serie,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return EquipmentFitmentDTO{}, fmt.Errorf("error scanning equipment fitment: %w", err)
	}

	applyEquipmentFitmentNullables(&fitment, model, serie, updatedBy, deletedAt)

	return fitment, nil
}

func scanEquipmentFitmentRow(row *sql.Row) (*EquipmentFitmentDTO, error) {
	var fitment EquipmentFitmentDTO
	var model sql.NullString
	var serie sql.NullString
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.EquipmentTypeID,
		&model,
		&serie,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEquipmentFitmentNotFound
		}
		return nil, fmt.Errorf("error scanning equipment fitment: %w", err)
	}

	applyEquipmentFitmentNullables(&fitment, model, serie, updatedBy, deletedAt)

	return &fitment, nil
}

func applyEquipmentFitmentNullables(fitment *EquipmentFitmentDTO, model, serie sql.NullString, updatedBy sql.NullInt64, deletedAt sql.NullTime) {
	if model.Valid {
		fitment.Model = &model.String
	}
	if serie.Valid {
		fitment.Serie = &serie.String
	}
	if updatedBy.Valid {
		fitment.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		fitment.DeletedAt = &deletedAt.Time
	}
}

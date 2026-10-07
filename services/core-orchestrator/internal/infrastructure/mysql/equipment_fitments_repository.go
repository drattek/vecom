package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEquipmentFitmentNotFound = errors.New("equipment fitment not found")
var ErrEquipmentFitmentInvalidReference = errors.New("equipment fitment invalid reference")

type EquipmentFitmentDTO struct {
	ID                int64   `json:"id"`
	BrandID           int64   `json:"brandId"`
	BrandName         string  `json:"brandName"`
	EquipmentTypeID   int64   `json:"equipmentTypeId"`
	EquipmentTypeName string  `json:"equipmentTypeName"`
	Model             *string `json:"model,omitempty"`
	Serie             *string `json:"serie,omitempty"`
	// ProductCount is how many products are currently linked to this fitment
	// (live rows of ecom_product_equipment_compatibility).
	ProductCount int64      `json:"productCount"`
	CreatedBy    int64      `json:"createdBy"`
	UpdatedBy    *int64     `json:"updatedBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedEquipmentFitments struct {
	Total    int                   `json:"total"`
	Offset   int                   `json:"offset"`
	PageSize int                   `json:"pageSize"`
	Fitments []EquipmentFitmentDTO `json:"fitments"`
}

// EquipmentFitmentFilter narrows FindPaginated. Query matches brand, type,
// model or serie.
type EquipmentFitmentFilter struct {
	Query           string
	BrandID         int64
	EquipmentTypeID int64
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

const equipmentFitmentFrom = `
	FROM ecom_equipment_fitment ef
	LEFT JOIN ecom_brands b ON b.id = ef.brand_id
	LEFT JOIN ecom_equipment_types et ON et.id = ef.equipment_type
`

const equipmentFitmentSelect = `
	SELECT ef.id, ef.brand_id, COALESCE(b.name, ''), ef.equipment_type, COALESCE(et.name, ''),
	       ef.model, ef.serie,
	       (SELECT COUNT(*) FROM ecom_product_equipment_compatibility pec
	         WHERE pec.equipment_fitment_id = ef.id AND pec.deleted_at IS NULL),
	       ef.created_by, ef.updated_by, ef.created_at, ef.updated_at, ef.deleted_at
` + equipmentFitmentFrom

func (r *EquipmentFitmentsRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedEquipmentFitments, error) {
	return r.Search(ctx, EquipmentFitmentFilter{}, offset, pageSize)
}

func (r *EquipmentFitmentsRepository) Search(ctx context.Context, filter EquipmentFitmentFilter, offset, pageSize int) (*PaginatedEquipmentFitments, error) {
	where := []string{"ef.deleted_at IS NULL"}
	args := []any{}

	if filter.BrandID > 0 {
		where = append(where, "ef.brand_id = ?")
		args = append(args, filter.BrandID)
	}
	if filter.EquipmentTypeID > 0 {
		where = append(where, "ef.equipment_type = ?")
		args = append(args, filter.EquipmentTypeID)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		pattern := likePattern(q)
		where = append(where, "(ef.model LIKE ? OR ef.serie LIKE ? OR b.name LIKE ? OR et.name LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) "+equipmentFitmentFrom+" WHERE "+whereSQL, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting equipment fitments: %w", err)
	}

	query := equipmentFitmentSelect + " WHERE " + whereSQL + " ORDER BY b.name ASC, et.name ASC, ef.model ASC, ef.serie ASC, ef.id ASC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, pageSize, offset)...)
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
	row := r.db.QueryRowContext(ctx, equipmentFitmentSelect+" WHERE ef.id = ? AND ef.deleted_at IS NULL LIMIT 1", id)
	return scanEquipmentFitmentRow(row)
}

// FindByUniqueKey looks up a live fitment by its natural key. model/serie are
// compared NULL-safely (<=>) since either may be absent; the table has no
// unique index for this (NULLs would defeat it), so callers must check here
// before inserting.
func (r *EquipmentFitmentsRepository) FindByUniqueKey(ctx context.Context, brandID, equipmentTypeID int64, model, serie *string) (*EquipmentFitmentDTO, error) {
	query := equipmentFitmentSelect + `
		WHERE ef.brand_id = ? AND ef.equipment_type = ? AND ef.model <=> ? AND ef.serie <=> ? AND ef.deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, brandID, equipmentTypeID, model, serie)
	return scanEquipmentFitmentRow(row)
}

func (r *EquipmentFitmentsRepository) Create(ctx context.Context, input CreateEquipmentFitmentInput) (*EquipmentFitmentDTO, error) {
	query := `
		INSERT INTO ecom_equipment_fitment (brand_id, equipment_type, model, serie, created_by, created_at, updated_at)
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
		UPDATE ecom_equipment_fitment
		SET brand_id = ?, equipment_type = ?, model = ?, serie = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.EquipmentTypeID, input.Model, input.Serie, input.UpdatedBy, id)
	if err != nil {
		if isForeignKeyConstraintError(err) {
			return nil, ErrEquipmentFitmentInvalidReference
		}
		return nil, fmt.Errorf("error updating equipment fitment: %w", err)
	}

	if _, err := result.RowsAffected(); err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	// MySQL reports 0 affected rows when the new values equal the old ones, so
	// FindByID (ErrEquipmentFitmentNotFound when absent) decides not-found.
	return r.FindByID(ctx, id)
}

func (r *EquipmentFitmentsRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_equipment_fitment
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

// SoftDeleteLinks soft-deletes every live product link of the fitment. Returns
// how many were removed.
func (r *EquipmentFitmentsRepository) SoftDeleteLinks(ctx context.Context, fitmentID, actorID int64) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_equipment_compatibility
		SET deleted_at = NOW(), updated_by = ?
		WHERE equipment_fitment_id = ? AND deleted_at IS NULL
	`, actorID, fitmentID)
	if err != nil {
		return 0, fmt.Errorf("error deleting product links of equipment fitment: %w", err)
	}
	return result.RowsAffected()
}

// CountByEquipmentType returns how many live fitments use the equipment type.
func (r *EquipmentFitmentsRepository) CountByEquipmentType(ctx context.Context, equipmentTypeID int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ecom_equipment_fitment WHERE equipment_type = ? AND deleted_at IS NULL",
		equipmentTypeID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting equipment fitments by type: %w", err)
	}
	return count, nil
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
		&fitment.BrandName,
		&fitment.EquipmentTypeID,
		&fitment.EquipmentTypeName,
		&model,
		&serie,
		&fitment.ProductCount,
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
		&fitment.BrandName,
		&fitment.EquipmentTypeID,
		&fitment.EquipmentTypeName,
		&model,
		&serie,
		&fitment.ProductCount,
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

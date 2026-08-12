package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrEquipmentTypeNotFound = errors.New("equipment type not found")

type EquipmentTypeDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedEquipmentTypes struct {
	Total          int64              `json:"total"`
	Offset         int                `json:"offset"`
	PageSize       int                `json:"pageSize"`
	EquipmentTypes []EquipmentTypeDTO `json:"equipmentTypes"`
}

type CreateEquipmentTypeInput struct {
	Name      string
	CreatedBy int64
}

type UpdateEquipmentTypeInput struct {
	Name      string
	UpdatedBy int64
}

type EquipmentTypesRepository struct {
	db *sql.DB
}

func NewEquipmentTypesRepository(db *sql.DB) *EquipmentTypesRepository {
	return &EquipmentTypesRepository{db: db}
}

func (r *EquipmentTypesRepository) FindPaginated(offset, pageSize int) (*PaginatedEquipmentTypes, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_equipment_types WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting equipment types: %w", err)
	}

	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_equipment_types
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying equipment types: %w", err)
	}
	defer rows.Close()

	equipmentTypes := make([]EquipmentTypeDTO, 0)
	for rows.Next() {
		equipmentType, scanErr := scanEquipmentType(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		equipmentTypes = append(equipmentTypes, equipmentType)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating equipment types: %w", err)
	}

	return &PaginatedEquipmentTypes{
		Total:          total,
		Offset:         offset,
		PageSize:       pageSize,
		EquipmentTypes: equipmentTypes,
	}, nil
}

func (r *EquipmentTypesRepository) FindByID(id int64) (*EquipmentTypeDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_equipment_types
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanEquipmentTypeRow(row)
}

func (r *EquipmentTypesRepository) Create(input CreateEquipmentTypeInput) (*EquipmentTypeDTO, error) {
	query := `
		INSERT INTO ecom_equipment_types (name, created_by, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.Name, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating equipment type: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *EquipmentTypesRepository) Update(id int64, input UpdateEquipmentTypeInput) (*EquipmentTypeDTO, error) {
	query := `
		UPDATE ecom_equipment_types
		SET name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Name, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating equipment type: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrEquipmentTypeNotFound
	}

	return r.FindByID(id)
}

func (r *EquipmentTypesRepository) SoftDelete(id int64) error {
	query := `
		UPDATE ecom_equipment_types
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting equipment type: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrEquipmentTypeNotFound
	}

	return nil
}

func scanEquipmentType(rows *sql.Rows) (EquipmentTypeDTO, error) {
	var equipmentType EquipmentTypeDTO
	err := rows.Scan(
		&equipmentType.ID,
		&equipmentType.Name,
		&equipmentType.CreatedBy,
		&equipmentType.UpdatedBy,
		&equipmentType.CreatedAt,
		&equipmentType.UpdatedAt,
		&equipmentType.DeletedAt,
	)
	if err != nil {
		return EquipmentTypeDTO{}, fmt.Errorf("error scanning equipment type: %w", err)
	}
	return equipmentType, nil
}

func scanEquipmentTypeRow(row *sql.Row) (*EquipmentTypeDTO, error) {
	var equipmentType EquipmentTypeDTO
	err := row.Scan(
		&equipmentType.ID,
		&equipmentType.Name,
		&equipmentType.CreatedBy,
		&equipmentType.UpdatedBy,
		&equipmentType.CreatedAt,
		&equipmentType.UpdatedAt,
		&equipmentType.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEquipmentTypeNotFound
		}
		return nil, fmt.Errorf("error scanning equipment type: %w", err)
	}
	return &equipmentType, nil
}

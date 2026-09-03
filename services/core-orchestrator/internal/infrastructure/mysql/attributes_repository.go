package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrAttributeNotFound = errors.New("attribute not found")

// AttributeDataTypes are the allowed values for ecom_attributes.data_type.
var AttributeDataTypes = []string{"text", "number", "boolean", "date", "enum"}

type AttributeDTO struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	DataType  string     `json:"dataType"`
	Unit      *string    `json:"unit,omitempty"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedAttributes struct {
	Total      int64          `json:"total"`
	Offset     int            `json:"offset"`
	PageSize   int            `json:"pageSize"`
	Attributes []AttributeDTO `json:"attributes"`
}

type CreateAttributeInput struct {
	Code      string
	Name      string
	DataType  string
	Unit      *string
	CreatedBy int64
}

// UpdateAttributeInput intentionally excludes Code: once a system_field/
// custom_attribute mapping in ecom_channel_attribute_map (or a product's
// ecom_product_attributes row) references an attribute, its code is meant to
// stay a stable identifier — only display metadata is editable.
type UpdateAttributeInput struct {
	Name      string
	DataType  string
	Unit      *string
	UpdatedBy int64
}

type AttributesRepository struct {
	db Querier
}

func NewAttributesRepository(db Querier) *AttributesRepository {
	return &AttributesRepository{db: db}
}

func (r *AttributesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedAttributes, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_attributes WHERE deleted_at IS NULL").Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting attributes: %w", err)
	}

	query := `
		SELECT id, code, name, data_type, unit, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attributes
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying attributes: %w", err)
	}
	defer rows.Close()

	attributes := make([]AttributeDTO, 0)
	for rows.Next() {
		attribute, scanErr := scanAttribute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attributes = append(attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attributes: %w", err)
	}

	return &PaginatedAttributes{
		Total:      total,
		Offset:     offset,
		PageSize:   pageSize,
		Attributes: attributes,
	}, nil
}

func (r *AttributesRepository) FindByID(ctx context.Context, id int64) (*AttributeDTO, error) {
	query := `
		SELECT id, code, name, data_type, unit, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attributes
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanAttributeRow(r.db.QueryRowContext(ctx, query, id))
}

func (r *AttributesRepository) FindByCode(ctx context.Context, code string) (*AttributeDTO, error) {
	query := `
		SELECT id, code, name, data_type, unit, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attributes
		WHERE code = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanAttributeRow(r.db.QueryRowContext(ctx, query, code))
}

func (r *AttributesRepository) Create(ctx context.Context, input CreateAttributeInput) (*AttributeDTO, error) {
	query := `
		INSERT INTO ecom_attributes (code, name, data_type, unit, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.Code, input.Name, input.DataType, input.Unit, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating attribute: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *AttributesRepository) Update(ctx context.Context, id int64, input UpdateAttributeInput) (*AttributeDTO, error) {
	query := `
		UPDATE ecom_attributes
		SET name = ?, data_type = ?, unit = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.DataType, input.Unit, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating attribute: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrAttributeNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *AttributesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `UPDATE ecom_attributes SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting attribute: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrAttributeNotFound
	}

	return nil
}

func scanAttribute(rows *sql.Rows) (AttributeDTO, error) {
	var attribute AttributeDTO
	err := rows.Scan(
		&attribute.ID,
		&attribute.Code,
		&attribute.Name,
		&attribute.DataType,
		&attribute.Unit,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		return AttributeDTO{}, fmt.Errorf("error scanning attribute: %w", err)
	}
	return attribute, nil
}

func scanAttributeRow(row *sql.Row) (*AttributeDTO, error) {
	var attribute AttributeDTO
	err := row.Scan(
		&attribute.ID,
		&attribute.Code,
		&attribute.Name,
		&attribute.DataType,
		&attribute.Unit,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttributeNotFound
		}
		return nil, fmt.Errorf("error scanning attribute: %w", err)
	}
	return &attribute, nil
}

package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrAttributeOptionNotFound = errors.New("attribute option not found")

type AttributeOptionDTO struct {
	ID          int64  `json:"id"`
	AttributeID int64  `json:"attributeId"`
	Value       string `json:"value"`
	// ExternalValueID is the channel's own id for this option (e.g.
	// MercadoLibre's value_id for a fixed-catalogue list attribute) — nil for
	// an option that only exists locally (free-text enum, never matched
	// against a channel's list). Populated by provisioning
	// (channel_attribute_values.ProvisionCategoryAttributes) when the
	// attribute's channel slot is value_id-mode; never set by the plain
	// /api/attribute-options CRUD endpoints.
	ExternalValueID *string    `json:"externalValueId,omitempty"`
	CreatedBy       int64      `json:"createdBy"`
	UpdatedBy       *int64     `json:"updatedBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
}

type CreateAttributeOptionInput struct {
	AttributeID int64
	Value       string
	// ExternalValueID is optional — set by provisioning for a channel-sourced
	// option, left nil for one created through the plain CRUD endpoint.
	ExternalValueID *string
	CreatedBy       int64
}

type UpdateAttributeOptionInput struct {
	Value     string
	UpdatedBy int64
}

type AttributeOptionsRepository struct {
	db *sql.DB
}

func NewAttributeOptionsRepository(db *sql.DB) *AttributeOptionsRepository {
	return &AttributeOptionsRepository{db: db}
}

func (r *AttributeOptionsRepository) FindByAttributeID(attributeID int64) ([]AttributeOptionDTO, error) {
	query := `
		SELECT id, attribute_id, value, external_value_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attribute_options
		WHERE attribute_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.Query(query, attributeID)
	if err != nil {
		return nil, fmt.Errorf("error querying attribute options: %w", err)
	}
	defer rows.Close()

	options := make([]AttributeOptionDTO, 0)
	for rows.Next() {
		option, scanErr := scanAttributeOption(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attribute options: %w", err)
	}

	return options, nil
}

func (r *AttributeOptionsRepository) FindByID(id int64) (*AttributeOptionDTO, error) {
	query := `
		SELECT id, attribute_id, value, external_value_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attribute_options
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanAttributeOptionRow(r.db.QueryRow(query, id))
}

// FindByAttributeAndValue looks up an existing option by its exact text under
// attributeID — used to avoid creating duplicate options (e.g. "Rojo" typed
// twice with different casing/spacing would otherwise fragment the same enum
// value into two rows).
func (r *AttributeOptionsRepository) FindByAttributeAndValue(attributeID int64, value string) (*AttributeOptionDTO, error) {
	query := `
		SELECT id, attribute_id, value, external_value_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attribute_options
		WHERE attribute_id = ? AND value = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanAttributeOptionRow(r.db.QueryRow(query, attributeID, value))
}

// FindByAttributeAndExternalValueID looks up an existing option by the
// channel's own id under attributeID — used by provisioning to key off
// MercadoLibre's stable value_id instead of its display name (which could in
// principle be renamed on MercadoLibre's side without the id changing).
func (r *AttributeOptionsRepository) FindByAttributeAndExternalValueID(attributeID int64, externalValueID string) (*AttributeOptionDTO, error) {
	query := `
		SELECT id, attribute_id, value, external_value_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_attribute_options
		WHERE attribute_id = ? AND external_value_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanAttributeOptionRow(r.db.QueryRow(query, attributeID, externalValueID))
}

func (r *AttributeOptionsRepository) Create(input CreateAttributeOptionInput) (*AttributeOptionDTO, error) {
	query := `
		INSERT INTO ecom_attribute_options (attribute_id, value, external_value_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.AttributeID, input.Value, input.ExternalValueID, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating attribute option: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *AttributeOptionsRepository) Update(id int64, input UpdateAttributeOptionInput) (*AttributeOptionDTO, error) {
	query := `
		UPDATE ecom_attribute_options
		SET value = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Value, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating attribute option: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrAttributeOptionNotFound
	}

	return r.FindByID(id)
}

func (r *AttributeOptionsRepository) SoftDelete(id int64) error {
	query := `UPDATE ecom_attribute_options SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting attribute option: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrAttributeOptionNotFound
	}

	return nil
}

func scanAttributeOption(rows *sql.Rows) (AttributeOptionDTO, error) {
	var option AttributeOptionDTO
	err := rows.Scan(
		&option.ID,
		&option.AttributeID,
		&option.Value,
		&option.ExternalValueID,
		&option.CreatedBy,
		&option.UpdatedBy,
		&option.CreatedAt,
		&option.UpdatedAt,
		&option.DeletedAt,
	)
	if err != nil {
		return AttributeOptionDTO{}, fmt.Errorf("error scanning attribute option: %w", err)
	}
	return option, nil
}

func scanAttributeOptionRow(row *sql.Row) (*AttributeOptionDTO, error) {
	var option AttributeOptionDTO
	err := row.Scan(
		&option.ID,
		&option.AttributeID,
		&option.Value,
		&option.ExternalValueID,
		&option.CreatedBy,
		&option.UpdatedBy,
		&option.CreatedAt,
		&option.UpdatedAt,
		&option.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttributeOptionNotFound
		}
		return nil, fmt.Errorf("error scanning attribute option: %w", err)
	}
	return &option, nil
}

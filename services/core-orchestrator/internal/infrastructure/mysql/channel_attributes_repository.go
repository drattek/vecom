package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrChannelAttributeNotFound = errors.New("channel attribute not found")

// ChannelAttributeTargetStrategies are the allowed values for
// ecom_channel_attributes.target_strategy. fixed_key is for channels with a
// known attribute vocabulary (e.g. MercadoLibre's BRAND, PART_NUMBER...);
// dynamic_field is for channels with no fixed keys, where the value is
// instead inserted into a named field/mechanism of the channel's own model
// (e.g. Odoo's attribute_line_ids).
var ChannelAttributeTargetStrategies = []string{"fixed_key", "dynamic_field"}

// ChannelAttributeValueModes are the allowed values for
// ecom_channel_attributes.value_mode, mirroring MercadoLibre's distinction
// between attributes set by value_name vs. value_id.
var ChannelAttributeValueModes = []string{"value_name", "value_id"}

type ChannelAttributeDTO struct {
	ID             int64      `json:"id"`
	ChannelID      int64      `json:"channelId"`
	TargetStrategy string     `json:"targetStrategy"`
	ExternalKey    *string    `json:"externalKey,omitempty"`
	TargetField    *string    `json:"targetField,omitempty"`
	ExternalLabel  *string    `json:"externalLabel,omitempty"`
	ValueMode      string     `json:"valueMode"`
	CategoryID     *int64     `json:"categoryId,omitempty"`
	IsRequired     bool       `json:"isRequired"`
	CreatedBy      int64      `json:"createdBy"`
	UpdatedBy      *int64     `json:"updatedBy,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedChannelAttributes struct {
	Total      int64                 `json:"total"`
	Offset     int                   `json:"offset"`
	PageSize   int                   `json:"pageSize"`
	Attributes []ChannelAttributeDTO `json:"attributes"`
}

type CreateChannelAttributeInput struct {
	ChannelID      int64
	TargetStrategy string
	ExternalKey    *string
	TargetField    *string
	ExternalLabel  *string
	ValueMode      string
	CategoryID     *int64
	IsRequired     bool
	CreatedBy      int64
}

type UpdateChannelAttributeInput struct {
	TargetStrategy string
	ExternalKey    *string
	TargetField    *string
	ExternalLabel  *string
	ValueMode      string
	CategoryID     *int64
	IsRequired     bool
	UpdatedBy      int64
}

type ChannelAttributesRepository struct {
	db *sql.DB
}

func NewChannelAttributesRepository(db *sql.DB) *ChannelAttributesRepository {
	return &ChannelAttributesRepository{db: db}
}

func (r *ChannelAttributesRepository) FindPaginated(offset, pageSize int) (*PaginatedChannelAttributes, error) {
	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_channel_attributes WHERE deleted_at IS NULL").Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting channel attributes: %w", err)
	}

	query := `
		SELECT id, channel_id, target_strategy, external_key, target_field, external_label, value_mode,
		       category_id, is_required, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attributes
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying channel attributes: %w", err)
	}
	defer rows.Close()

	attributes := make([]ChannelAttributeDTO, 0)
	for rows.Next() {
		attribute, scanErr := scanChannelAttribute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attributes = append(attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel attributes: %w", err)
	}

	return &PaginatedChannelAttributes{
		Total:      total,
		Offset:     offset,
		PageSize:   pageSize,
		Attributes: attributes,
	}, nil
}

func (r *ChannelAttributesRepository) FindByID(id int64) (*ChannelAttributeDTO, error) {
	query := `
		SELECT id, channel_id, target_strategy, external_key, target_field, external_label, value_mode,
		       category_id, is_required, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attributes
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelAttributeRow(r.db.QueryRow(query, id))
}

func (r *ChannelAttributesRepository) FindByChannelID(channelID int64) ([]ChannelAttributeDTO, error) {
	query := `
		SELECT id, channel_id, target_strategy, external_key, target_field, external_label, value_mode,
		       category_id, is_required, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attributes
		WHERE channel_id = ? AND deleted_at IS NULL
		ORDER BY category_id IS NULL DESC, id ASC
	`

	rows, err := r.db.Query(query, channelID)
	if err != nil {
		return nil, fmt.Errorf("error querying channel attributes: %w", err)
	}
	defer rows.Close()

	attributes := make([]ChannelAttributeDTO, 0)
	for rows.Next() {
		attribute, scanErr := scanChannelAttribute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attributes = append(attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel attributes: %w", err)
	}

	return attributes, nil
}

// FindApplicable returns every ecom_channel_attributes slot a listing under
// categoryID should fill on channelID: rows scoped to that exact category
// plus every channel-wide row (category_id IS NULL). Category-specific rows
// are ordered first so a resolver can let them take precedence over a
// same-external_key generic row. categoryID may be nil (product has no
// category yet), in which case only the generic rows are returned. Not
// called anywhere yet — added so the future attribute resolver (replacing
// the hardcoded MercadoLibre/Odoo attribute lists) has the read path ready.
func (r *ChannelAttributesRepository) FindApplicable(channelID int64, categoryID *int64) ([]ChannelAttributeDTO, error) {
	query := `
		SELECT id, channel_id, target_strategy, external_key, target_field, external_label, value_mode,
		       category_id, is_required, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attributes
		WHERE channel_id = ? AND deleted_at IS NULL AND (category_id IS NULL OR category_id = ?)
		ORDER BY category_id IS NULL ASC, id ASC
	`

	rows, err := r.db.Query(query, channelID, categoryID)
	if err != nil {
		return nil, fmt.Errorf("error querying applicable channel attributes: %w", err)
	}
	defer rows.Close()

	attributes := make([]ChannelAttributeDTO, 0)
	for rows.Next() {
		attribute, scanErr := scanChannelAttribute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attributes = append(attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating applicable channel attributes: %w", err)
	}

	return attributes, nil
}

func (r *ChannelAttributesRepository) Create(input CreateChannelAttributeInput) (*ChannelAttributeDTO, error) {
	query := `
		INSERT INTO ecom_channel_attributes
			(channel_id, target_strategy, external_key, target_field, external_label, value_mode, category_id, is_required, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.ChannelID, input.TargetStrategy, input.ExternalKey, input.TargetField, input.ExternalLabel, input.ValueMode, input.CategoryID, input.IsRequired, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating channel attribute: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *ChannelAttributesRepository) Update(id int64, input UpdateChannelAttributeInput) (*ChannelAttributeDTO, error) {
	query := `
		UPDATE ecom_channel_attributes
		SET target_strategy = ?, external_key = ?, target_field = ?, external_label = ?, value_mode = ?,
		    category_id = ?, is_required = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.TargetStrategy, input.ExternalKey, input.TargetField, input.ExternalLabel, input.ValueMode, input.CategoryID, input.IsRequired, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating channel attribute: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrChannelAttributeNotFound
	}

	return r.FindByID(id)
}

func (r *ChannelAttributesRepository) SoftDelete(id int64) error {
	query := `UPDATE ecom_channel_attributes SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting channel attribute: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrChannelAttributeNotFound
	}

	return nil
}

func scanChannelAttribute(rows *sql.Rows) (ChannelAttributeDTO, error) {
	var attribute ChannelAttributeDTO
	err := rows.Scan(
		&attribute.ID,
		&attribute.ChannelID,
		&attribute.TargetStrategy,
		&attribute.ExternalKey,
		&attribute.TargetField,
		&attribute.ExternalLabel,
		&attribute.ValueMode,
		&attribute.CategoryID,
		&attribute.IsRequired,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		return ChannelAttributeDTO{}, fmt.Errorf("error scanning channel attribute: %w", err)
	}
	return attribute, nil
}

func scanChannelAttributeRow(row *sql.Row) (*ChannelAttributeDTO, error) {
	var attribute ChannelAttributeDTO
	err := row.Scan(
		&attribute.ID,
		&attribute.ChannelID,
		&attribute.TargetStrategy,
		&attribute.ExternalKey,
		&attribute.TargetField,
		&attribute.ExternalLabel,
		&attribute.ValueMode,
		&attribute.CategoryID,
		&attribute.IsRequired,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelAttributeNotFound
		}
		return nil, fmt.Errorf("error scanning channel attribute: %w", err)
	}
	return &attribute, nil
}

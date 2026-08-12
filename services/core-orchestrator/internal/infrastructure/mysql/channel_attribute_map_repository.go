package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrChannelAttributeMapNotFound = errors.New("channel attribute map not found")

// ChannelAttributeMapSourceTypes are the allowed values for
// ecom_channel_attribute_map.source_type: where the value that fills a
// channel attribute slot comes from — a real dynamic product attribute, an
// existing domain field (brand, part number, sku, dimensions...), or a fixed
// business value (e.g. MercadoLibre's ITEM_CONDITION).
var ChannelAttributeMapSourceTypes = []string{"custom_attribute", "system_field", "static_value"}

type ChannelAttributeMapDTO struct {
	ID                 int64      `json:"id"`
	ChannelAttributeID int64      `json:"channelAttributeId"`
	ConnectionID       *int64     `json:"connectionId,omitempty"`
	SourceType         string     `json:"sourceType"`
	AttributeID        *int64     `json:"attributeId,omitempty"`
	SystemField        *string    `json:"systemField,omitempty"`
	StaticValue        *string    `json:"staticValue,omitempty"`
	CreatedBy          int64      `json:"createdBy"`
	UpdatedBy          *int64     `json:"updatedBy,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
}

type CreateChannelAttributeMapInput struct {
	ChannelAttributeID int64
	ConnectionID       *int64
	SourceType         string
	AttributeID        *int64
	SystemField        *string
	StaticValue        *string
	CreatedBy          int64
}

type UpdateChannelAttributeMapInput struct {
	SourceType  string
	AttributeID *int64
	SystemField *string
	StaticValue *string
	UpdatedBy   int64
}

type ChannelAttributeMapRepository struct {
	db *sql.DB
}

func NewChannelAttributeMapRepository(db *sql.DB) *ChannelAttributeMapRepository {
	return &ChannelAttributeMapRepository{db: db}
}

func (r *ChannelAttributeMapRepository) FindByChannelAttributeID(channelAttributeID int64) ([]ChannelAttributeMapDTO, error) {
	query := `
		SELECT id, channel_attribute_id, connection_id, source_type, attribute_id, system_field, static_value,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attribute_map
		WHERE channel_attribute_id = ? AND deleted_at IS NULL
		ORDER BY connection_id IS NULL ASC, id ASC
	`

	rows, err := r.db.Query(query, channelAttributeID)
	if err != nil {
		return nil, fmt.Errorf("error querying channel attribute map: %w", err)
	}
	defer rows.Close()

	maps := make([]ChannelAttributeMapDTO, 0)
	for rows.Next() {
		m, scanErr := scanChannelAttributeMap(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		maps = append(maps, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel attribute map: %w", err)
	}

	return maps, nil
}

// FindByChannelAttributeAndConnection resolves which map row applies to a
// given (channel attribute slot, connection) pair: an exact connection_id
// match wins over the channel-wide row (connection_id IS NULL). Not called
// anywhere yet — this is the exact-then-generic lookup the future attribute
// resolver needs once ecom_channel_attribute_map is populated.
func (r *ChannelAttributeMapRepository) FindByChannelAttributeAndConnection(channelAttributeID int64, connectionID int64) (*ChannelAttributeMapDTO, error) {
	query := `
		SELECT id, channel_attribute_id, connection_id, source_type, attribute_id, system_field, static_value,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attribute_map
		WHERE channel_attribute_id = ? AND deleted_at IS NULL AND (connection_id = ? OR connection_id IS NULL)
		ORDER BY connection_id IS NULL ASC
		LIMIT 1
	`

	return scanChannelAttributeMapRow(r.db.QueryRow(query, channelAttributeID, connectionID))
}

func (r *ChannelAttributeMapRepository) FindByID(id int64) (*ChannelAttributeMapDTO, error) {
	query := `
		SELECT id, channel_attribute_id, connection_id, source_type, attribute_id, system_field, static_value,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_channel_attribute_map
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanChannelAttributeMapRow(r.db.QueryRow(query, id))
}

func (r *ChannelAttributeMapRepository) Create(input CreateChannelAttributeMapInput) (*ChannelAttributeMapDTO, error) {
	query := `
		INSERT INTO ecom_channel_attribute_map
			(channel_attribute_id, connection_id, source_type, attribute_id, system_field, static_value, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.ChannelAttributeID, input.ConnectionID, input.SourceType, input.AttributeID, input.SystemField, input.StaticValue, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating channel attribute map: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *ChannelAttributeMapRepository) Update(id int64, input UpdateChannelAttributeMapInput) (*ChannelAttributeMapDTO, error) {
	query := `
		UPDATE ecom_channel_attribute_map
		SET source_type = ?, attribute_id = ?, system_field = ?, static_value = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.SourceType, input.AttributeID, input.SystemField, input.StaticValue, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating channel attribute map: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrChannelAttributeMapNotFound
	}

	return r.FindByID(id)
}

func (r *ChannelAttributeMapRepository) SoftDelete(id int64) error {
	query := `UPDATE ecom_channel_attribute_map SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting channel attribute map: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrChannelAttributeMapNotFound
	}

	return nil
}

func scanChannelAttributeMap(rows *sql.Rows) (ChannelAttributeMapDTO, error) {
	var m ChannelAttributeMapDTO
	err := rows.Scan(
		&m.ID,
		&m.ChannelAttributeID,
		&m.ConnectionID,
		&m.SourceType,
		&m.AttributeID,
		&m.SystemField,
		&m.StaticValue,
		&m.CreatedBy,
		&m.UpdatedBy,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		return ChannelAttributeMapDTO{}, fmt.Errorf("error scanning channel attribute map: %w", err)
	}
	return m, nil
}

func scanChannelAttributeMapRow(row *sql.Row) (*ChannelAttributeMapDTO, error) {
	var m ChannelAttributeMapDTO
	err := row.Scan(
		&m.ID,
		&m.ChannelAttributeID,
		&m.ConnectionID,
		&m.SourceType,
		&m.AttributeID,
		&m.SystemField,
		&m.StaticValue,
		&m.CreatedBy,
		&m.UpdatedBy,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelAttributeMapNotFound
		}
		return nil, fmt.Errorf("error scanning channel attribute map: %w", err)
	}
	return &m, nil
}

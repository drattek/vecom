package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrChannelNotFound = errors.New("channel not found")
var ErrChannelCodeAlreadyExists = errors.New("channel code already exists")
var ErrChannelInvalidReference = errors.New("channel invalid reference")

type ChannelDTO struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Code            string     `json:"code"`
	Status          string     `json:"status"`
	IconID          *int64     `json:"iconId,omitempty"`
	Description     *string    `json:"description,omitempty"`
	ConnectionCount int64      `json:"connectionCount"`
	CreatedBy       int64      `json:"createdBy"`
	UpdatedBy       *int64     `json:"updatedBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedChannels struct {
	Total    int64        `json:"total"`
	Offset   int          `json:"offset"`
	PageSize int          `json:"pageSize"`
	Channels []ChannelDTO `json:"channels"`
}

type CreateChannelInput struct {
	Name        string
	Code        string
	Status      string
	IconID      *int64
	Description *string
	CreatedBy   int64
}

type UpdateChannelInput struct {
	Name        string
	Code        string
	Status      string
	IconID      *int64
	Description *string
	UpdatedBy   int64
}

type ChannelRepository struct {
	db Querier
}

func NewChannelRepository(db Querier) *ChannelRepository {
	return &ChannelRepository{db: db}
}

func (r *ChannelRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedChannels, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_channels WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting channels: %w", err)
	}

	query := `
		SELECT
			c.id,
			c.name,
			c.code,
			c.status,
			c.icon_id,
			c.description,
			COALESCE(cc.connection_count, 0) AS connection_count,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_channels c
		LEFT JOIN (
			SELECT channel_id, COUNT(*) AS connection_count
			FROM ecom_channel_connections
			WHERE deleted_at IS NULL
			GROUP BY channel_id
		) cc ON cc.channel_id = c.id
		WHERE c.deleted_at IS NULL
		ORDER BY c.id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying channels: %w", err)
	}
	defer rows.Close()

	channels := make([]ChannelDTO, 0)
	for rows.Next() {
		channel, scanErr := scanChannel(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		channels = append(channels, channel)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return &PaginatedChannels{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Channels: channels,
	}, nil
}

// FindAll returns every non-deleted channel ordered by id — the unpaginated
// counterpart of FindPaginated, for callers that need to cross-reference every
// channel (e.g. category_import.Service grouping active connections by their
// channel code).
func (r *ChannelRepository) FindAll(ctx context.Context) ([]ChannelDTO, error) {
	query := `
		SELECT
			c.id,
			c.name,
			c.code,
			c.status,
			c.icon_id,
			c.description,
			COALESCE(cc.connection_count, 0) AS connection_count,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_channels c
		LEFT JOIN (
			SELECT channel_id, COUNT(*) AS connection_count
			FROM ecom_channel_connections
			WHERE deleted_at IS NULL
			GROUP BY channel_id
		) cc ON cc.channel_id = c.id
		WHERE c.deleted_at IS NULL
		ORDER BY c.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying channels: %w", err)
	}
	defer rows.Close()

	channels := make([]ChannelDTO, 0)
	for rows.Next() {
		channel, scanErr := scanChannel(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		channels = append(channels, channel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

func (r *ChannelRepository) FindByID(ctx context.Context, id int64) (*ChannelDTO, error) {
	query := `
		SELECT
			c.id,
			c.name,
			c.code,
			c.status,
			c.icon_id,
			c.description,
			COALESCE(cc.connection_count, 0) AS connection_count,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_channels c
		LEFT JOIN (
			SELECT channel_id, COUNT(*) AS connection_count
			FROM ecom_channel_connections
			WHERE deleted_at IS NULL
			GROUP BY channel_id
		) cc ON cc.channel_id = c.id
		WHERE c.id = ? AND c.deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	channel, err := scanChannel(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}

	return &channel, nil
}

func (r *ChannelRepository) FindByCode(ctx context.Context, code string) (*ChannelDTO, error) {
	query := `
		SELECT
			c.id,
			c.name,
			c.code,
			c.status,
			c.icon_id,
			c.description,
			COALESCE(cc.connection_count, 0) AS connection_count,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_channels c
		LEFT JOIN (
			SELECT channel_id, COUNT(*) AS connection_count
			FROM ecom_channel_connections
			WHERE deleted_at IS NULL
			GROUP BY channel_id
		) cc ON cc.channel_id = c.id
		WHERE c.code = ? AND c.deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, code)
	channel, err := scanChannel(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}

	return &channel, nil
}

func (r *ChannelRepository) Create(ctx context.Context, input CreateChannelInput) (*ChannelDTO, error) {
	query := `
		INSERT INTO ecom_channels (name, code, status, icon_id, description, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, NULL)
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.Name,
		input.Code,
		input.Status,
		normalizedInt64OrNil(input.IconID),
		normalizedStringOrNil(input.Description),
		input.CreatedBy,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrChannelCodeAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrChannelInvalidReference
		}
		return nil, fmt.Errorf("error creating channel: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting channel id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelRepository) Update(ctx context.Context, id int64, input UpdateChannelInput) (*ChannelDTO, error) {
	query := `
		UPDATE ecom_channels
		SET name = ?, code = ?, status = ?, icon_id = ?, description = ?, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.Name,
		input.Code,
		input.Status,
		normalizedInt64OrNil(input.IconID),
		normalizedStringOrNil(input.Description),
		input.UpdatedBy,
		id,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrChannelCodeAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrChannelInvalidReference
		}
		return nil, fmt.Errorf("error updating channel: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error checking updated rows: %w", err)
	}

	if affected == 0 {
		return nil, ErrChannelNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelRepository) SoftDelete(ctx context.Context, id, updatedBy int64) error {
	query := `
		UPDATE ecom_channels
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error soft deleting channel: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking deleted rows: %w", err)
	}

	if affected == 0 {
		return ErrChannelNotFound
	}

	return nil
}

func scanChannel(scanner interface{ Scan(dest ...any) error }) (ChannelDTO, error) {
	var channel ChannelDTO
	var iconID sql.NullInt64
	var description sql.NullString
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&channel.ID,
		&channel.Name,
		&channel.Code,
		&channel.Status,
		&iconID,
		&description,
		&channel.ConnectionCount,
		&channel.CreatedBy,
		&updatedBy,
		&channel.CreatedAt,
		&channel.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return ChannelDTO{}, fmt.Errorf("error scanning channel: %w", err)
	}

	if iconID.Valid {
		channel.IconID = &iconID.Int64
	}

	if description.Valid {
		trimmedDescription := strings.TrimSpace(description.String)
		if trimmedDescription != "" {
			channel.Description = &trimmedDescription
		}
	}

	if updatedBy.Valid {
		channel.UpdatedBy = &updatedBy.Int64
	}

	if deletedAt.Valid {
		channel.DeletedAt = &deletedAt.Time
	}

	return channel, nil
}

func normalizedInt64OrNil(value *int64) interface{} {
	if value == nil {
		return nil
	}

	if *value <= 0 {
		return nil
	}

	return *value
}

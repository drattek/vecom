package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrChannelParameterNotFound = errors.New("channel parameter not found")
)

type ChannelParameterDTO struct {
	ID            int64     `json:"id"`
	ChannelID     int64     `json:"channelId"`
	ParameterName string    `json:"parameterName"`
	DisplayName   string    `json:"displayName"`
	ParameterType string    `json:"parameterType"`
	Required      bool      `json:"required"`
	DefaultValue  *string   `json:"defaultValue"`
	IsEncrypted   bool      `json:"isEncrypted"`
	CreatedBy     int64     `json:"createdBy"`
	UpdatedBy     *int64    `json:"updatedBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type PaginatedChannelParameters struct {
	Data  []ChannelParameterDTO `json:"data"`
	Total int                   `json:"total"`
}

type CreateChannelParameterInput struct {
	ChannelID     int64
	ParameterName string
	DisplayName   string
	ParameterType string
	Required      bool
	DefaultValue  *string
	IsEncrypted   bool
	CreatedBy     int64
}

type UpdateChannelParameterInput struct {
	DisplayName   string
	ParameterType string
	Required      bool
	DefaultValue  *string
	IsEncrypted   bool
	UpdatedBy     int64
}

type ChannelParametersRepository struct {
	db Querier
}

func NewChannelParametersRepository(db Querier) *ChannelParametersRepository {
	return &ChannelParametersRepository{db: db}
}

func (r *ChannelParametersRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedChannelParameters, error) {
	query := `
		SELECT id, channel_id, parameter_name, display_name, parameter_type, required, 
		       default_value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_channel_parameters
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parameters []ChannelParameterDTO
	for rows.Next() {
		var c ChannelParameterDTO
		if err := rows.Scan(&c.ID, &c.ChannelID, &c.ParameterName, &c.DisplayName, &c.ParameterType, &c.Required,
			&c.DefaultValue, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		parameters = append(parameters, c)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_channel_parameters WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedChannelParameters{Data: parameters, Total: total}, nil
}

func (r *ChannelParametersRepository) FindByID(ctx context.Context, id int64) (*ChannelParameterDTO, error) {
	query := `
		SELECT id, channel_id, parameter_name, display_name, parameter_type, required, 
		       default_value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_channel_parameters
		WHERE id = ? AND deleted_at IS NULL
	`

	var c ChannelParameterDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&c.ID, &c.ChannelID, &c.ParameterName, &c.DisplayName, &c.ParameterType, &c.Required,
		&c.DefaultValue, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrChannelParameterNotFound
		}
		return nil, err
	}

	return &c, nil
}

func (r *ChannelParametersRepository) FindByChannelID(ctx context.Context, channelID int64, offset, pageSize int) (*PaginatedChannelParameters, error) {
	query := `
		SELECT id, channel_id, parameter_name, display_name, parameter_type, required, 
		       default_value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_channel_parameters
		WHERE channel_id = ? AND deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, channelID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parameters []ChannelParameterDTO
	for rows.Next() {
		var c ChannelParameterDTO
		if err := rows.Scan(&c.ID, &c.ChannelID, &c.ParameterName, &c.DisplayName, &c.ParameterType, &c.Required,
			&c.DefaultValue, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		parameters = append(parameters, c)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_channel_parameters WHERE channel_id = ? AND deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, channelID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedChannelParameters{Data: parameters, Total: total}, nil
}

func (r *ChannelParametersRepository) Create(ctx context.Context, input CreateChannelParameterInput) (*ChannelParameterDTO, error) {
	query := `
		INSERT INTO ecom_channel_parameters (channel_id, parameter_name, display_name, parameter_type, 
		                                      required, default_value, is_encrypted, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ChannelID, input.ParameterName, input.DisplayName, input.ParameterType,
		input.Required, input.DefaultValue, input.IsEncrypted, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelParametersRepository) Update(ctx context.Context, id int64, input UpdateChannelParameterInput) (*ChannelParameterDTO, error) {
	query := `
		UPDATE ecom_channel_parameters
		SET display_name = ?, parameter_type = ?, required = ?, default_value = ?, 
		    is_encrypted = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.DisplayName, input.ParameterType, input.Required, input.DefaultValue,
		input.IsEncrypted, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrChannelParameterNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelParametersRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_channel_parameters SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrChannelParameterNotFound
	}

	return nil
}

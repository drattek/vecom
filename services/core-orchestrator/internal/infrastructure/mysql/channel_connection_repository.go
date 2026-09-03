package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

var ErrChannelConnectionNotFound = errors.New("channel connection not found")
var ErrChannelConnectionInvalidReference = errors.New("channel connection invalid reference")

type ChannelConnectionDTO struct {
	ID          int64  `json:"id"`
	ChannelID   int64  `json:"channelId"`
	ChannelName string `json:"channelName"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Environment string `json:"environment"`
	CurrencyID  int64  `json:"currencyId"`
	// AllowsMultipleListings gates whether product sync flows may create more
	// than one external listing per (product, connection) — e.g. MercadoLibre
	// connections that publish one listing per vehicle compatibility. Single-
	// listing connections (Odoo, most MercadoLibre connections) keep this false.
	AllowsMultipleListings bool       `json:"allowsMultipleListings"`
	CreatedBy              int64      `json:"createdBy"`
	UpdatedBy              *int64     `json:"updatedBy,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	DeletedAt              *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedChannelConnections struct {
	Total              int64                  `json:"total"`
	Offset             int                    `json:"offset"`
	PageSize           int                    `json:"pageSize"`
	ChannelConnections []ChannelConnectionDTO `json:"channelConnections"`
}

type CreateChannelConnectionInput struct {
	ChannelID              int64
	Name                   string
	Status                 string
	Environment            string
	CurrencyID             int64
	AllowsMultipleListings bool
	CreatedBy              int64
}

type UpdateChannelConnectionInput struct {
	ChannelID              int64
	Name                   string
	Status                 string
	Environment            string
	CurrencyID             int64
	AllowsMultipleListings bool
	UpdatedBy              int64
}

type ChannelConnectionRepository struct {
	db Querier
}

func NewChannelConnectionRepository(db Querier) *ChannelConnectionRepository {
	return &ChannelConnectionRepository{db: db}
}

func (r *ChannelConnectionRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedChannelConnections, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_channel_connections WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting channel connections: %w", err)
	}

	query := `
		SELECT cc.id, cc.channel_id, cc.name, cc.status, cc.environment, cc.currency_id,
		       cc.allows_multiple_listings, cc.created_by, cc.updated_by, cc.created_at, cc.updated_at, cc.deleted_at,
		       ch.name
		FROM ecom_channel_connections cc
		LEFT JOIN ecom_channels ch ON ch.id = cc.channel_id
		WHERE cc.deleted_at IS NULL
		ORDER BY cc.id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying channel connections: %w", err)
	}
	defer rows.Close()

	channelConnections := make([]ChannelConnectionDTO, 0)
	for rows.Next() {
		channelConnection, scanErr := scanChannelConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		channelConnections = append(channelConnections, channelConnection)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating channel connections: %w", err)
	}

	return &PaginatedChannelConnections{
		Total:              total,
		Offset:             offset,
		PageSize:           pageSize,
		ChannelConnections: channelConnections,
	}, nil
}

// FindActiveByChannelCode returns every non-deleted, status='active'
// connection whose channel matches channelCode (ecom_channels.code, matched
// case-insensitively) — for schedulers that need to iterate every connection
// of one specific channel (e.g. CompatibilitiesFixScheduler), unlike
// FindPaginated which lists every connection regardless of channel.
func (r *ChannelConnectionRepository) FindActiveByChannelCode(ctx context.Context, channelCode string) ([]ChannelConnectionDTO, error) {
	query := `
		SELECT cc.id, cc.channel_id, cc.name, cc.status, cc.environment, cc.currency_id,
		       cc.allows_multiple_listings, cc.created_by, cc.updated_by, cc.created_at, cc.updated_at, cc.deleted_at,
		       ch.name
		FROM ecom_channel_connections cc
		JOIN ecom_channels ch ON ch.id = cc.channel_id
		WHERE cc.deleted_at IS NULL AND cc.status = 'active' AND UPPER(TRIM(ch.code)) = UPPER(TRIM(?))
		ORDER BY cc.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, channelCode)
	if err != nil {
		return nil, fmt.Errorf("error querying channel connections for channel code %q: %w", channelCode, err)
	}
	defer rows.Close()

	channelConnections := make([]ChannelConnectionDTO, 0)
	for rows.Next() {
		channelConnection, scanErr := scanChannelConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		channelConnections = append(channelConnections, channelConnection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel connections for channel code %q: %w", channelCode, err)
	}

	return channelConnections, nil
}

func (r *ChannelConnectionRepository) FindByID(ctx context.Context, id int64) (*ChannelConnectionDTO, error) {
	query := `
		SELECT cc.id, cc.channel_id, cc.name, cc.status, cc.environment, cc.currency_id,
		       cc.allows_multiple_listings, cc.created_by, cc.updated_by, cc.created_at, cc.updated_at, cc.deleted_at,
		       ch.name
		FROM ecom_channel_connections cc
		LEFT JOIN ecom_channels ch ON ch.id = cc.channel_id
		WHERE cc.id = ? AND cc.deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	channelConnection, err := scanChannelConnection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChannelConnectionNotFound
		}
		return nil, err
	}

	return &channelConnection, nil
}

func (r *ChannelConnectionRepository) Create(ctx context.Context, input CreateChannelConnectionInput) (*ChannelConnectionDTO, error) {
	query := `
		INSERT INTO ecom_channel_connections (channel_id, name, status, environment, currency_id, allows_multiple_listings, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.ChannelID,
		input.Name,
		input.Status,
		input.Environment,
		input.CurrencyID,
		input.AllowsMultipleListings,
		input.CreatedBy,
	)
	if err != nil {
		if isForeignKeyConstraintError(err) {
			return nil, ErrChannelConnectionInvalidReference
		}
		return nil, fmt.Errorf("error creating channel connection: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting channel connection id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelConnectionRepository) Update(ctx context.Context, id int64, input UpdateChannelConnectionInput) (*ChannelConnectionDTO, error) {
	query := `
		UPDATE ecom_channel_connections
		SET channel_id = ?, name = ?, status = ?, environment = ?, currency_id = ?, allows_multiple_listings = ?, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.ChannelID,
		input.Name,
		input.Status,
		input.Environment,
		input.CurrencyID,
		input.AllowsMultipleListings,
		input.UpdatedBy,
		id,
	)
	if err != nil {
		if isForeignKeyConstraintError(err) {
			return nil, ErrChannelConnectionInvalidReference
		}
		return nil, fmt.Errorf("error updating channel connection: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error checking updated rows: %w", err)
	}

	if affected == 0 {
		return nil, ErrChannelConnectionNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ChannelConnectionRepository) SoftDelete(ctx context.Context, id, updatedBy int64) error {
	query := `
		UPDATE ecom_channel_connections
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error soft deleting channel connection: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking deleted rows: %w", err)
	}

	if affected == 0 {
		return ErrChannelConnectionNotFound
	}

	return nil
}

func scanChannelConnection(scanner interface{ Scan(dest ...any) error }) (ChannelConnectionDTO, error) {
	var channelConnection ChannelConnectionDTO
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime
	var channelName sql.NullString

	err := scanner.Scan(
		&channelConnection.ID,
		&channelConnection.ChannelID,
		&channelConnection.Name,
		&channelConnection.Status,
		&channelConnection.Environment,
		&channelConnection.CurrencyID,
		&channelConnection.AllowsMultipleListings,
		&channelConnection.CreatedBy,
		&updatedBy,
		&channelConnection.CreatedAt,
		&channelConnection.UpdatedAt,
		&deletedAt,
		&channelName,
	)
	if err != nil {
		return ChannelConnectionDTO{}, fmt.Errorf("error scanning channel connection: %w", err)
	}

	channelConnection.ChannelName = channelName.String

	if updatedBy.Valid {
		channelConnection.UpdatedBy = &updatedBy.Int64
	}

	if deletedAt.Valid {
		channelConnection.DeletedAt = &deletedAt.Time
	}

	return channelConnection, nil
}

func isForeignKeyConstraintError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}

	return mysqlErr.Number == 1452
}

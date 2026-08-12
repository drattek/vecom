package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrConnectionStatusNotFound = errors.New("connection status not found")

type ConnectionStatusDTO struct {
	ID            int64      `json:"id"`
	ConnectionID  int64      `json:"connectionId"`
	Authenticated bool       `json:"authenticated"`
	LastAuth      *time.Time `json:"lastAuth"`
	LastError     *string    `json:"lastError"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	LastSync      *time.Time `json:"lastSync"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// UpsertConnectionStatusInput records the outcome of an authentication/refresh
// (or sync) attempt for a connection. LastAuth, ExpiresAt and LastSync are only
// applied when non-nil; a nil pointer keeps the previously stored value.
// LastError is always applied as-is (pass nil to clear a previous error).
type UpsertConnectionStatusInput struct {
	ConnectionID  int64
	Authenticated bool
	LastError     *string
	LastAuth      *time.Time
	ExpiresAt     *time.Time
	LastSync      *time.Time
}

type ConnectionStatusRepository struct {
	db *sql.DB
}

func NewConnectionStatusRepository(db *sql.DB) *ConnectionStatusRepository {
	return &ConnectionStatusRepository{db: db}
}

func (r *ConnectionStatusRepository) FindByConnectionID(connectionID int64) (*ConnectionStatusDTO, error) {
	query := `
		SELECT id, connection_id, authenticated, last_auth, last_error, expires_at, last_sync, created_at, updated_at
		FROM ecom_connection_status
		WHERE connection_id = ?
	`

	return scanConnectionStatusRow(r.db.QueryRow(query, connectionID))
}

func (r *ConnectionStatusRepository) findByID(id int64) (*ConnectionStatusDTO, error) {
	query := `
		SELECT id, connection_id, authenticated, last_auth, last_error, expires_at, last_sync, created_at, updated_at
		FROM ecom_connection_status
		WHERE id = ?
	`

	return scanConnectionStatusRow(r.db.QueryRow(query, id))
}

// Upsert creates the status row for a connection on first write, or updates it
// on subsequent writes, preserving LastAuth/ExpiresAt/LastSync when the caller
// does not provide a new value for them.
func (r *ConnectionStatusRepository) Upsert(input UpsertConnectionStatusInput) (*ConnectionStatusDTO, error) {
	existing, err := r.FindByConnectionID(input.ConnectionID)
	if err != nil && !errors.Is(err, ErrConnectionStatusNotFound) {
		return nil, fmt.Errorf("error loading connection status: %w", err)
	}

	lastAuth := input.LastAuth
	expiresAt := input.ExpiresAt
	lastSync := input.LastSync

	if existing != nil {
		if lastAuth == nil {
			lastAuth = existing.LastAuth
		}
		if expiresAt == nil {
			expiresAt = existing.ExpiresAt
		}
		if lastSync == nil {
			lastSync = existing.LastSync
		}
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_connection_status (connection_id, authenticated, last_auth, last_error, expires_at, last_sync)
			VALUES (?, ?, ?, ?, ?, ?)
		`

		result, err := r.db.Exec(query, input.ConnectionID, input.Authenticated, lastAuth, input.LastError, expiresAt, lastSync)
		if err != nil {
			return nil, fmt.Errorf("error creating connection status: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(id)
	}

	query := `
		UPDATE ecom_connection_status
		SET authenticated = ?, last_auth = ?, last_error = ?, expires_at = ?, last_sync = ?, updated_at = NOW()
		WHERE id = ?
	`

	if _, err := r.db.Exec(query, input.Authenticated, lastAuth, input.LastError, expiresAt, lastSync, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating connection status: %w", err)
	}

	return r.findByID(existing.ID)
}

// DueTokenRefreshDTO identifies a connection whose token is close to expiring,
// together with the code of the channel it belongs to (ecom_channels.code),
// so a scheduler can dispatch the refresh to the right marketplace handler.
type DueTokenRefreshDTO struct {
	ConnectionID int64     `json:"connectionId"`
	ChannelCode  string    `json:"channelCode"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// FindDueForRefresh returns connections whose stored token expires at or
// before threshold. Connections with no expires_at (e.g. API-key based
// integrations like Odoo, which have nothing to refresh) and connections/
// channels that are soft-deleted or not active are excluded.
func (r *ConnectionStatusRepository) FindDueForRefresh(threshold time.Time) ([]DueTokenRefreshDTO, error) {
	query := `
		SELECT cs.connection_id, ch.code, cs.expires_at
		FROM ecom_connection_status cs
		INNER JOIN ecom_channel_connections cc ON cc.id = cs.connection_id
		INNER JOIN ecom_channels ch ON ch.id = cc.channel_id
		WHERE cs.expires_at IS NOT NULL
			AND cs.expires_at <= ?
			AND cc.deleted_at IS NULL
			AND cc.status = 'active'
			AND ch.status = 'active'
	`

	rows, err := r.db.Query(query, threshold)
	if err != nil {
		return nil, fmt.Errorf("error querying connections due for token refresh: %w", err)
	}
	defer rows.Close()

	due := make([]DueTokenRefreshDTO, 0)
	for rows.Next() {
		var d DueTokenRefreshDTO
		if err := rows.Scan(&d.ConnectionID, &d.ChannelCode, &d.ExpiresAt); err != nil {
			return nil, fmt.Errorf("error scanning due token refresh row: %w", err)
		}
		due = append(due, d)
	}

	return due, rows.Err()
}

func scanConnectionStatusRow(row *sql.Row) (*ConnectionStatusDTO, error) {
	var s ConnectionStatusDTO
	err := row.Scan(
		&s.ID,
		&s.ConnectionID,
		&s.Authenticated,
		&s.LastAuth,
		&s.LastError,
		&s.ExpiresAt,
		&s.LastSync,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConnectionStatusNotFound
		}
		return nil, fmt.Errorf("error scanning connection status: %w", err)
	}

	return &s, nil
}

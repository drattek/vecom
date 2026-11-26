package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Control del job de ventas, una fila por conexión (ADR 0010).

type SaleSyncStateDTO struct {
	ConnectionID  int64
	FloorAt       time.Time
	CursorAt      *time.Time
	LastStartedAt *time.Time
	LastSuccessAt *time.Time
}

// SaleSyncStatusDTO es el estado de una conexión que importa ventas, exista o
// no todavía su fila de control.
type SaleSyncStatusDTO struct {
	ConnectionID    int64      `json:"connectionId"`
	ConnectionName  string     `json:"connectionName"`
	ChannelCode     string     `json:"channelCode"`
	ChannelName     string     `json:"channelName"`
	Active          bool       `json:"active"`
	Running         bool       `json:"running"`
	InitialLoadDone bool       `json:"initialLoadDone"`
	LastStartedAt   *time.Time `json:"lastStartedAt"`
	LastFinishedAt  *time.Time `json:"lastFinishedAt"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt"`
	LastError       *string    `json:"lastError"`
	LastSavedCount  int        `json:"lastSavedCount"`
	LastFailedCount int        `json:"lastFailedCount"`
}

// SaleSyncResult es el cierre de una corrida. CursorAt solo se informa cuando
// la corrida recorrió y guardó todo.
type SaleSyncResult struct {
	CursorAt    *time.Time
	Error       *string
	SavedCount  int
	FailedCount int
}

type SaleSyncStateRepository struct {
	db Querier
}

func NewSaleSyncStateRepository(db Querier) *SaleSyncStateRepository {
	return &SaleSyncStateRepository{db: db}
}

// Ensure crea la fila de control si no existe. floorAt solo aplica al crearla:
// el piso queda fijo desde la primera corrida.
func (r *SaleSyncStateRepository) Ensure(ctx context.Context, connectionID int64, floorAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_sale_sync_state (connection_id, floor_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE connection_id = connection_id`,
		connectionID, floorAt,
	)
	if err != nil {
		return fmt.Errorf("error creating sale sync state: %w", err)
	}
	return nil
}

// Find devuelve nil, nil si la conexión todavía no tiene fila de control.
func (r *SaleSyncStateRepository) Find(ctx context.Context, connectionID int64) (*SaleSyncStateDTO, error) {
	var state SaleSyncStateDTO
	err := r.db.QueryRowContext(ctx, `
		SELECT connection_id, floor_at, cursor_at, last_started_at, last_success_at
		FROM ecom_sale_sync_state WHERE connection_id = ?`, connectionID,
	).Scan(&state.ConnectionID, &state.FloorAt, &state.CursorAt, &state.LastStartedAt, &state.LastSuccessAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading sale sync state: %w", err)
	}
	return &state, nil
}

// AcquireLock toma el bloqueo si está libre o vencido. Evita dos corridas
// simultáneas de la misma conexión, también entre réplicas.
func (r *SaleSyncStateRepository) AcquireLock(ctx context.Context, connectionID int64, token string, now, until time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ecom_sale_sync_state
		SET lock_token = ?, locked_until = ?, last_started_at = ?
		WHERE connection_id = ? AND (locked_until IS NULL OR locked_until < ?)`,
		token, until, now, connectionID, now,
	)
	if err != nil {
		return false, fmt.Errorf("error acquiring sale sync lock: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error checking sale sync lock: %w", err)
	}
	return affected == 1, nil
}

// RenewLock extiende el bloqueo; false significa que ya no es nuestro.
func (r *SaleSyncStateRepository) RenewLock(ctx context.Context, connectionID int64, token string, until time.Time) (bool, error) {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE ecom_sale_sync_state SET locked_until = ? WHERE connection_id = ? AND lock_token = ?`,
		until, connectionID, token,
	); err != nil {
		return false, fmt.Errorf("error renewing sale sync lock: %w", err)
	}
	// RowsAffected es 0 si el valor no cambió, así que se comprueba aparte.
	var owned int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ecom_sale_sync_state WHERE connection_id = ? AND lock_token = ?`,
		connectionID, token,
	).Scan(&owned); err != nil {
		return false, fmt.Errorf("error verifying sale sync lock: %w", err)
	}
	return owned == 1, nil
}

// Finish registra el resultado y libera el bloqueo.
func (r *SaleSyncStateRepository) Finish(ctx context.Context, connectionID int64, token string, now time.Time, result SaleSyncResult) error {
	var successAt *time.Time
	if result.CursorAt != nil {
		successAt = &now
	}
	var lastError *string
	if result.Error != nil {
		clipped := truncate(*result.Error, 1000)
		lastError = &clipped
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_sale_sync_state
		SET cursor_at = COALESCE(?, cursor_at), last_success_at = COALESCE(?, last_success_at),
		    last_finished_at = ?, last_error = ?, last_saved_count = ?, last_failed_count = ?,
		    lock_token = NULL, locked_until = NULL
		WHERE connection_id = ? AND lock_token = ?`,
		result.CursorAt, successAt, now, lastError, result.SavedCount, result.FailedCount, connectionID, token,
	)
	if err != nil {
		return fmt.Errorf("error finishing sale sync run: %w", err)
	}
	return nil
}

// ListStatus devuelve las conexiones de los canales indicados, más las que ya
// no están activas pero conservan ventas importadas (para poder filtrarlas).
func (r *SaleSyncStateRepository) ListStatus(ctx context.Context, channelCodes []string, now time.Time) ([]SaleSyncStatusDTO, error) {
	if len(channelCodes) == 0 {
		return []SaleSyncStatusDTO{}, nil
	}
	args := []any{now}
	for _, code := range channelCodes {
		args = append(args, strings.ToUpper(code))
	}
	query := fmt.Sprintf(`
		SELECT cc.id, cc.name, ch.code, ch.name,
		       (cc.deleted_at IS NULL AND cc.status = 'active'),
		       COALESCE(st.locked_until > ?, 0), (st.cursor_at IS NOT NULL),
		       st.last_started_at, st.last_finished_at, st.last_success_at, st.last_error,
		       COALESCE(st.last_saved_count, 0), COALESCE(st.last_failed_count, 0)
		FROM ecom_channel_connections cc
		JOIN ecom_channels ch ON ch.id = cc.channel_id
		LEFT JOIN ecom_sale_sync_state st ON st.connection_id = cc.id
		WHERE UPPER(TRIM(ch.code)) IN (%s)
		  AND ((cc.deleted_at IS NULL AND cc.status = 'active') OR st.id IS NOT NULL)
		ORDER BY ch.name ASC, cc.name ASC`, placeholders(len(channelCodes)))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying sale sync status: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, s *SaleSyncStatusDTO) error {
		return rows.Scan(&s.ConnectionID, &s.ConnectionName, &s.ChannelCode, &s.ChannelName,
			&s.Active, &s.Running, &s.InitialLoadDone,
			&s.LastStartedAt, &s.LastFinishedAt, &s.LastSuccessAt, &s.LastError,
			&s.LastSavedCount, &s.LastFailedCount)
	})
}

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ErpSyncStateDTO es la última actividad de un flujo del ERP.
type ErpSyncStateDTO struct {
	FeedCode        string
	LastCompletedAt *time.Time
	CreatedAt       time.Time
}

// ErpSyncStateRepository guarda ecom_erp_sync_state, base del aviso por falta
// de sincronización.
type ErpSyncStateRepository struct {
	db Querier
}

func NewErpSyncStateRepository(db Querier) *ErpSyncStateRepository {
	return &ErpSyncStateRepository{db: db}
}

func (r *ErpSyncStateRepository) RecordCompleted(ctx context.Context, feedCode string, total, failed int, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_erp_sync_state (feed_code, last_completed_at, last_total_count, last_failed_count)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_completed_at = VALUES(last_completed_at),
			last_total_count = VALUES(last_total_count),
			last_failed_count = VALUES(last_failed_count),
			deleted_at = NULL
	`, feedCode, at, total, failed)
	if err != nil {
		return fmt.Errorf("recording completed erp sync %s: %w", feedCode, err)
	}
	return nil
}

func (r *ErpSyncStateRepository) RecordFailed(ctx context.Context, feedCode, message string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_erp_sync_state (feed_code, last_failed_at, last_error)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_failed_at = VALUES(last_failed_at),
			last_error = VALUES(last_error),
			deleted_at = NULL
	`, feedCode, at, truncateRunes(message, 1000))
	if err != nil {
		return fmt.Errorf("recording failed erp sync %s: %w", feedCode, err)
	}
	return nil
}

func (r *ErpSyncStateRepository) List(ctx context.Context) ([]ErpSyncStateDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT feed_code, last_completed_at, created_at
		FROM ecom_erp_sync_state
		WHERE deleted_at IS NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing erp sync state: %w", err)
	}
	defer rows.Close()

	states := make([]ErpSyncStateDTO, 0, 3)
	for rows.Next() {
		var (
			state       ErpSyncStateDTO
			completedAt sql.NullTime
		)
		if err := rows.Scan(&state.FeedCode, &completedAt, &state.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading erp sync state: %w", err)
		}
		if completedAt.Valid {
			state.LastCompletedAt = &completedAt.Time
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

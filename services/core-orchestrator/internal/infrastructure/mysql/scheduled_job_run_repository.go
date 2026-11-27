package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ScheduledJobRunRepository (ecom_scheduled_job_runs) impide ejecutar dos veces
// el mismo horario de una tarea, también tras un reinicio.
type ScheduledJobRunRepository struct {
	db Querier
}

func NewScheduledJobRunRepository(db Querier) *ScheduledJobRunRepository {
	return &ScheduledJobRunRepository{db: db}
}

// Claim toma la corrida de ese horario. Devuelve false si ya terminó o si otra
// ejecución la tiene tomada. Una corrida fallida o abandonada se puede retomar
// cuando vence su bloqueo.
func (r *ScheduledJobRunRepository) Claim(ctx context.Context, jobCode string, scheduledFor, now time.Time, lockFor time.Duration) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_scheduled_job_runs (job_code, scheduled_for, status, started_at, locked_until)
		VALUES (?, ?, 'running', ?, ?)
		ON DUPLICATE KEY UPDATE job_code = job_code
	`, jobCode, scheduledFor, now, now.Add(lockFor))
	if err != nil {
		return false, fmt.Errorf("claiming job %s: %w", jobCode, err)
	}
	if inserted, err := result.RowsAffected(); err != nil {
		return false, fmt.Errorf("claiming job %s: %w", jobCode, err)
	} else if inserted == 1 {
		return true, nil
	}

	result, err = r.db.ExecContext(ctx, `
		UPDATE ecom_scheduled_job_runs
		SET status = 'running', started_at = ?, locked_until = ?, finished_at = NULL
		WHERE job_code = ? AND scheduled_for = ? AND status <> 'done' AND locked_until < ?
	`, now, now.Add(lockFor), jobCode, scheduledFor, now)
	if err != nil {
		return false, fmt.Errorf("reclaiming job %s: %w", jobCode, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("reclaiming job %s: %w", jobCode, err)
	}
	return updated == 1, nil
}

func (r *ScheduledJobRunRepository) Complete(ctx context.Context, jobCode string, scheduledFor, finishedAt time.Time, windowFrom, windowTo *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_scheduled_job_runs
		SET status = 'done', finished_at = ?, locked_until = NULL, last_error = NULL, window_from = ?, window_to = ?
		WHERE job_code = ? AND scheduled_for = ?
	`, finishedAt, windowFrom, windowTo, jobCode, scheduledFor)
	if err != nil {
		return fmt.Errorf("completing job %s: %w", jobCode, err)
	}
	return nil
}

// Fail deja la corrida fallida; se podrá reintentar a partir de retryAt.
func (r *ScheduledJobRunRepository) Fail(ctx context.Context, jobCode string, scheduledFor, retryAt time.Time, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_scheduled_job_runs
		SET status = 'failed', locked_until = ?, last_error = ?
		WHERE job_code = ? AND scheduled_for = ?
	`, retryAt, truncateRunes(reason, 1000), jobCode, scheduledFor)
	if err != nil {
		return fmt.Errorf("failing job %s: %w", jobCode, err)
	}
	return nil
}

func (r *ScheduledJobRunRepository) IsDone(ctx context.Context, jobCode string, scheduledFor time.Time) (bool, error) {
	var found int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM ecom_scheduled_job_runs
		WHERE job_code = ? AND scheduled_for = ? AND status = 'done'
	`, jobCode, scheduledFor).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading job %s: %w", jobCode, err)
	}
	return true, nil
}

// LastWindowEnd es hasta dónde llegó la última corrida terminada; nil si no hay.
func (r *ScheduledJobRunRepository) LastWindowEnd(ctx context.Context, jobCode string) (*time.Time, error) {
	var windowTo sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT window_to FROM ecom_scheduled_job_runs
		WHERE job_code = ? AND status = 'done' AND window_to IS NOT NULL
		ORDER BY window_to DESC LIMIT 1
	`, jobCode).Scan(&windowTo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading last window of job %s: %w", jobCode, err)
	}
	if !windowTo.Valid {
		return nil, nil
	}
	return &windowTo.Time, nil
}

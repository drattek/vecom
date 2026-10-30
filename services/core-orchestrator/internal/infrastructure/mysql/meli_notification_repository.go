package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrMeliNotificationNotFound = errors.New("meli notification not found")

type MeliNotificationDTO struct {
	ID               int64      `json:"id"`
	NotificationID   string     `json:"notificationId"`
	Resource         string     `json:"resource"`
	Topic            string     `json:"topic"`
	MeliUserID       int64      `json:"meliUserId"`
	ApplicationID    int64      `json:"applicationId"`
	DeliveryAttempts int        `json:"deliveryAttempts"`
	MeliSentAt       *time.Time `json:"meliSentAt,omitempty"`
	MeliReceivedAt   *time.Time `json:"meliReceivedAt,omitempty"`
	IngestedAt       time.Time  `json:"ingestedAt"`
	ConnectionID     *int64     `json:"connectionId,omitempty"`
	RawPayload       string     `json:"rawPayload"`
	ProcessedPayload *string    `json:"processedPayload,omitempty"`
	Status           string     `json:"status"`
	LastError        *string    `json:"lastError,omitempty"`
	ProcessedAt      *time.Time `json:"processedAt,omitempty"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type CreateMeliNotificationInput struct {
	NotificationID   string
	Resource         string
	Topic            string
	MeliUserID       int64
	ApplicationID    int64
	DeliveryAttempts int
	MeliSentAt       *time.Time
	MeliReceivedAt   *time.Time
	RawPayload       string
}

type MeliNotificationRepository struct {
	db Querier
}

func NewMeliNotificationRepository(db Querier) *MeliNotificationRepository {
	return &MeliNotificationRepository{db: db}
}

const meliNotificationColumns = `
	id, notification_id, resource, topic, meli_user_id, application_id, delivery_attempts,
	meli_sent_at, meli_received_at, ingested_at, connection_id, raw_payload, processed_payload,
	status, last_error, processed_at, updated_by, created_at, updated_at, deleted_at
`

// FindByNotificationID looks up a previously stored notification by
// MercadoLibre's own `_id`, used by the application layer to make Create
// idempotent against MercadoLibre's at-least-once redelivery.
func (r *MeliNotificationRepository) FindByNotificationID(ctx context.Context, notificationID string) (*MeliNotificationDTO, error) {
	query := `
		SELECT ` + meliNotificationColumns + `
		FROM ecom_meli_notifications
		WHERE notification_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, notificationID)
	notification, err := scanMeliNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMeliNotificationNotFound
		}
		return nil, err
	}

	return &notification, nil
}

func (r *MeliNotificationRepository) FindByID(ctx context.Context, id int64) (*MeliNotificationDTO, error) {
	query := `
		SELECT ` + meliNotificationColumns + `
		FROM ecom_meli_notifications
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	notification, err := scanMeliNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMeliNotificationNotFound
		}
		return nil, err
	}

	return &notification, nil
}

func (r *MeliNotificationRepository) Create(ctx context.Context, input CreateMeliNotificationInput) (*MeliNotificationDTO, error) {
	query := `
		INSERT INTO ecom_meli_notifications
			(notification_id, resource, topic, meli_user_id, application_id, delivery_attempts, meli_sent_at, meli_received_at, raw_payload, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.NotificationID,
		input.Resource,
		input.Topic,
		input.MeliUserID,
		input.ApplicationID,
		input.DeliveryAttempts,
		input.MeliSentAt,
		input.MeliReceivedAt,
		input.RawPayload,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating meli notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting meli notification id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func scanMeliNotification(scanner interface{ Scan(dest ...any) error }) (MeliNotificationDTO, error) {
	var n MeliNotificationDTO
	var meliSentAt sql.NullTime
	var meliReceivedAt sql.NullTime
	var connectionID sql.NullInt64
	var processedPayload sql.NullString
	var lastError sql.NullString
	var processedAt sql.NullTime
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&n.ID,
		&n.NotificationID,
		&n.Resource,
		&n.Topic,
		&n.MeliUserID,
		&n.ApplicationID,
		&n.DeliveryAttempts,
		&meliSentAt,
		&meliReceivedAt,
		&n.IngestedAt,
		&connectionID,
		&n.RawPayload,
		&processedPayload,
		&n.Status,
		&lastError,
		&processedAt,
		&updatedBy,
		&n.CreatedAt,
		&n.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return MeliNotificationDTO{}, fmt.Errorf("error scanning meli notification: %w", err)
	}

	if meliSentAt.Valid {
		n.MeliSentAt = &meliSentAt.Time
	}
	if meliReceivedAt.Valid {
		n.MeliReceivedAt = &meliReceivedAt.Time
	}
	if connectionID.Valid {
		n.ConnectionID = &connectionID.Int64
	}
	if processedPayload.Valid {
		n.ProcessedPayload = &processedPayload.String
	}
	if lastError.Valid {
		n.LastError = &lastError.String
	}
	if processedAt.Valid {
		n.ProcessedAt = &processedAt.Time
	}
	if updatedBy.Valid {
		n.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		n.DeletedAt = &deletedAt.Time
	}

	return n, nil
}

// FindPending returns up to `limit` notifications with status='pending',
// oldest first. The processor loops on this to drain the backlog without
// starving brand-new webhooks.
func (r *MeliNotificationRepository) FindPending(ctx context.Context, limit int) ([]MeliNotificationDTO, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT ` + meliNotificationColumns + `
		FROM ecom_meli_notifications
		WHERE status = 'pending' AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying pending meli notifications: %w", err)
	}
	defer rows.Close()

	var out []MeliNotificationDTO
	for rows.Next() {
		n, err := scanMeliNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending meli notifications: %w", err)
	}
	return out, nil
}

// UpdateConnectionID stamps the connection the processor resolved from
// meli_user_id. Called separately from MarkDone so the connection stays
// stamped even if later topic handling fails.
func (r *MeliNotificationRepository) UpdateConnectionID(ctx context.Context, id, connectionID int64) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE ecom_meli_notifications SET connection_id = ? WHERE id = ?`,
		connectionID, id,
	); err != nil {
		return fmt.Errorf("error stamping connection_id on meli notification %d: %w", id, err)
	}
	return nil
}

// MarkDone transitions a notification to status='done' and persists the
// JSON MELI returned when we GET'd the resource. processedPayload may be
// empty for topics the shadow processor hasn't learned to resolve yet;
// those still go to 'done' with an explanatory last_error so they don't
// get retried forever.
func (r *MeliNotificationRepository) MarkDone(ctx context.Context, id int64, processedPayload, note string) error {
	var payloadArg interface{}
	if processedPayload != "" {
		payloadArg = processedPayload
	}
	var noteArg interface{}
	if note != "" {
		noteArg = note
	}

	if _, err := r.db.ExecContext(ctx,
		`UPDATE ecom_meli_notifications
		 SET status = 'done',
		     processed_payload = ?,
		     last_error = ?,
		     processed_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		payloadArg, noteArg, id,
	); err != nil {
		return fmt.Errorf("error marking meli notification %d done: %w", id, err)
	}
	return nil
}

// MarkFailed stamps the error and bumps attempts counter semantics by
// re-using delivery_attempts (local, not what MELI reports). Keeps the
// row at status='pending' so the next poll retries it; the processor's
// outer loop is responsible for backoff.
//
// If maxAttempts is reached, flips status to 'failed' so it stops cycling.
func (r *MeliNotificationRepository) MarkFailed(ctx context.Context, id int64, errMsg string, maxAttempts int) error {
	// Pull current attempts; use delivery_attempts as the local retry counter
	// too — MercadoLibre delivered the row once (its own retries happen
	// before it hits us), and the shadow processor's own retries are added on
	// top.
	//
	// A dedicated local_attempts column would be cleaner but keeps the
	// schema stable; delivery_attempts is already monotonic per row.
	query := `
		UPDATE ecom_meli_notifications
		SET delivery_attempts = delivery_attempts + 1,
		    last_error = ?,
		    status = CASE WHEN delivery_attempts + 1 >= ? THEN 'failed' ELSE 'pending' END
		WHERE id = ?
	`
	if _, err := r.db.ExecContext(ctx, query, errMsg, maxAttempts, id); err != nil {
		return fmt.Errorf("error marking meli notification %d failed: %w", id, err)
	}
	return nil
}

// CountByStatus is used by health checks / dashboard summaries. Not
// production-critical; returns a map of status → count ignoring soft-
// deleted rows.
func (r *MeliNotificationRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM ecom_meli_notifications WHERE deleted_at IS NULL GROUP BY status`,
	)
	if err != nil {
		return nil, fmt.Errorf("error counting meli notifications by status: %w", err)
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

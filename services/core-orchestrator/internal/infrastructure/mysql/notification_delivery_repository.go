package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NotificationDeliveryDTO es un envío en cola para un usuario.
type NotificationDeliveryDTO struct {
	ID                int64
	UserID            int64
	NotificationType  string
	Message           string
	AttachmentName    string
	AttachmentContent []byte
	Attempts          int
}

type EnqueueNotificationInput struct {
	UserID            int64
	NotificationType  string
	DedupeKey         string
	Message           string
	AttachmentName    string
	AttachmentContent []byte
}

// NotificationDeliveryRepository es la cola de envíos
// (ecom_notification_deliveries).
type NotificationDeliveryRepository struct {
	db Querier
}

func NewNotificationDeliveryRepository(db Querier) *NotificationDeliveryRepository {
	return &NotificationDeliveryRepository{db: db}
}

// Enqueue no hace nada si el usuario ya tiene un envío con esa dedupe_key.
func (r *NotificationDeliveryRepository) Enqueue(ctx context.Context, input EnqueueNotificationInput) error {
	var attachment any
	if len(input.AttachmentContent) > 0 {
		attachment = input.AttachmentContent
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_notification_deliveries
			(user_id, notification_type, dedupe_key, message, attachment_name, attachment_content)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key
	`, input.UserID, input.NotificationType, input.DedupeKey, input.Message,
		nullableString(input.AttachmentName), attachment)
	if err != nil {
		return fmt.Errorf("enqueuing notification for user %d: %w", input.UserID, err)
	}
	return nil
}

func (r *NotificationDeliveryRepository) Exists(ctx context.Context, userID int64, dedupeKey string) (bool, error) {
	var found int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM ecom_notification_deliveries WHERE user_id = ? AND dedupe_key = ? LIMIT 1
	`, userID, dedupeKey).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up notification delivery: %w", err)
	}
	return true, nil
}

// ExpireStale cancela lo que lleva más de maxAge sin poder enviarse, para no
// soltar de golpe avisos viejos (p. ej. tras tener apagadas las notificaciones).
func (r *NotificationDeliveryRepository) ExpireStale(ctx context.Context, maxAge time.Duration) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_deliveries
		SET status = 'cancelled', attachment_content = NULL, last_error = 'expired before it could be sent'
		WHERE status = 'pending' AND created_at < NOW() - INTERVAL ? SECOND AND deleted_at IS NULL
	`, int64(maxAge.Seconds()))
	if err != nil {
		return fmt.Errorf("expiring stale notifications: %w", err)
	}
	return nil
}

// ClaimNext toma el siguiente envío listo y lo deja 'sending' por lockFor.
// Devuelve (nil, nil) si no hay. Requiere un repositorio de transacción
// (mysql.WithinTx).
func (r *NotificationDeliveryRepository) ClaimNext(ctx context.Context, now time.Time, lockFor time.Duration) (*NotificationDeliveryDTO, error) {
	var (
		dto            NotificationDeliveryDTO
		attachmentName sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, notification_type, message, attachment_name, attachment_content, attempts
		FROM ecom_notification_deliveries
		WHERE deleted_at IS NULL
		  AND ((status = 'pending' AND (next_attempt_at IS NULL OR next_attempt_at <= ?))
		    OR (status = 'sending' AND locked_until < ?))
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, now, now).Scan(
		&dto.ID, &dto.UserID, &dto.NotificationType, &dto.Message,
		&attachmentName, &dto.AttachmentContent, &dto.Attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claiming notification delivery: %w", err)
	}
	dto.AttachmentName = attachmentName.String

	if _, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_deliveries
		SET status = 'sending', attempts = attempts + 1, locked_until = ?
		WHERE id = ?
	`, now.Add(lockFor), dto.ID); err != nil {
		return nil, fmt.Errorf("locking notification delivery %d: %w", dto.ID, err)
	}
	dto.Attempts++
	return &dto, nil
}

// MarkSent libera el adjunto: ya no se necesita.
func (r *NotificationDeliveryRepository) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_deliveries
		SET status = 'sent', sent_at = ?, locked_until = NULL, next_attempt_at = NULL,
		    last_error = NULL, attachment_content = NULL
		WHERE id = ?
	`, sentAt, id)
	if err != nil {
		return fmt.Errorf("marking notification %d sent: %w", id, err)
	}
	return nil
}

func (r *NotificationDeliveryRepository) Retry(ctx context.Context, id int64, nextAttempt time.Time, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_deliveries
		SET status = 'pending', locked_until = NULL, next_attempt_at = ?, last_error = ?
		WHERE id = ?
	`, nextAttempt, truncateRunes(reason, 1000), id)
	if err != nil {
		return fmt.Errorf("rescheduling notification %d: %w", id, err)
	}
	return nil
}

// Close termina un envío sin éxito: status es 'failed' o 'cancelled'.
func (r *NotificationDeliveryRepository) Close(ctx context.Context, id int64, status, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_deliveries
		SET status = ?, locked_until = NULL, next_attempt_at = NULL, last_error = ?, attachment_content = NULL
		WHERE id = ?
	`, status, truncateRunes(reason, 1000), id)
	if err != nil {
		return fmt.Errorf("closing notification %d: %w", id, err)
	}
	return nil
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

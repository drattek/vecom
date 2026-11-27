package mysql

import (
	"context"
	"fmt"
)

// NotificationPreferenceRepository guarda los avisos que cada usuario activa.
// Sin fila, el aviso está apagado.
type NotificationPreferenceRepository struct {
	db Querier
}

func NewNotificationPreferenceRepository(db Querier) *NotificationPreferenceRepository {
	return &NotificationPreferenceRepository{db: db}
}

// FindByUserID devuelve tipo → activado.
func (r *NotificationPreferenceRepository) FindByUserID(ctx context.Context, userID int64) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT notification_type, is_enabled
		FROM ecom_notification_preferences
		WHERE user_id = ? AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing notification preferences: %w", err)
	}
	defer rows.Close()

	preferences := make(map[string]bool)
	for rows.Next() {
		var (
			notificationType string
			enabled          bool
		)
		if err := rows.Scan(&notificationType, &enabled); err != nil {
			return nil, fmt.Errorf("reading notification preference: %w", err)
		}
		preferences[notificationType] = enabled
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing notification preferences: %w", err)
	}
	return preferences, nil
}

func (r *NotificationPreferenceRepository) Upsert(ctx context.Context, userID int64, notificationType string, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_notification_preferences (user_id, notification_type, is_enabled, created_by)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE is_enabled = VALUES(is_enabled), updated_by = VALUES(created_by), deleted_at = NULL
	`, userID, notificationType, enabled, userID)
	if err != nil {
		return fmt.Errorf("saving notification preference %s: %w", notificationType, err)
	}
	return nil
}

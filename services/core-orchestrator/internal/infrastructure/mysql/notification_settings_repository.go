package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotificationSettingsNotFound indica que falta la fila única que crea
// 2026-11-25_telegram_notifications.sql.
var ErrNotificationSettingsNotFound = errors.New("notification settings row not found")

// NotificationSettingsDTO es la configuración global de las notificaciones
// (ADR 0012). BotTokenEncrypted nunca sale por la API.
type NotificationSettingsDTO struct {
	BotTokenEncrypted string
	BotUsername       string
	IsEnabled         bool
	// DailySummaryTime en formato "HH:MM:SS".
	DailySummaryTime       string
	Timezone               string
	ErpSilenceHours        int
	ConnectionCheckMinutes int
	TelegramUpdateOffset   int64
}

type UpdateNotificationSettingsInput struct {
	BotTokenEncrypted      string
	BotUsername            string
	IsEnabled              bool
	DailySummaryTime       string
	ErpSilenceHours        int
	ConnectionCheckMinutes int
	TelegramUpdateOffset   int64
	ActorID                int64
}

type NotificationSettingsRepository struct {
	db Querier
}

func NewNotificationSettingsRepository(db Querier) *NotificationSettingsRepository {
	return &NotificationSettingsRepository{db: db}
}

func (r *NotificationSettingsRepository) Get(ctx context.Context) (*NotificationSettingsDTO, error) {
	var (
		dto             NotificationSettingsDTO
		token, username sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT bot_token_encrypted, bot_username, is_enabled, daily_summary_time, timezone,
		       erp_silence_hours, connection_check_minutes, telegram_update_offset
		FROM ecom_notification_settings
		WHERE singleton_key = 1 AND deleted_at IS NULL
	`).Scan(
		&token, &username, &dto.IsEnabled, &dto.DailySummaryTime, &dto.Timezone,
		&dto.ErpSilenceHours, &dto.ConnectionCheckMinutes, &dto.TelegramUpdateOffset,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotificationSettingsNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading notification settings: %w", err)
	}

	dto.BotTokenEncrypted = token.String
	dto.BotUsername = username.String
	return &dto, nil
}

func (r *NotificationSettingsRepository) Update(ctx context.Context, input UpdateNotificationSettingsInput) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_settings
		SET bot_token_encrypted = ?, bot_username = ?, is_enabled = ?, daily_summary_time = ?,
		    erp_silence_hours = ?, connection_check_minutes = ?, telegram_update_offset = ?, updated_by = ?
		WHERE singleton_key = 1 AND deleted_at IS NULL
	`,
		nullableString(input.BotTokenEncrypted), nullableString(input.BotUsername), input.IsEnabled,
		input.DailySummaryTime, input.ErpSilenceHours, input.ConnectionCheckMinutes,
		input.TelegramUpdateOffset, input.ActorID,
	)
	if err != nil {
		return fmt.Errorf("updating notification settings: %w", err)
	}
	return nil
}

// SaveUpdateOffset guarda el último update_id procesado, solo si el bot sigue
// siendo el mismo: una consulta en curso no debe pisar el offset de un bot nuevo.
func (r *NotificationSettingsRepository) SaveUpdateOffset(ctx context.Context, botUsername string, offset int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_settings
		SET telegram_update_offset = ?
		WHERE singleton_key = 1 AND bot_username = ? AND deleted_at IS NULL
	`, offset, botUsername)
	if err != nil {
		return fmt.Errorf("saving telegram update offset: %w", err)
	}
	return nil
}

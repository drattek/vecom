package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Estados de ecom_user_telegram_links.status.
const (
	TelegramLinkPending  = "pending"
	TelegramLinkLinked   = "linked"
	TelegramLinkBlocked  = "blocked"
	TelegramLinkUnlinked = "unlinked"
)

type TelegramLinkDTO struct {
	ID                   int64
	UserID               int64
	Username             string // usuario del dashboard (ecom_api_user)
	Status               string
	NotificationsEnabled bool
	ChatID               *int64
	TelegramUsername     string
	TelegramName         string
	BotUsername          string
	LinkCodeExpiresAt    *time.Time
	LinkedAt             *time.Time
}

type CompleteTelegramLinkInput struct {
	ID               int64
	CodeHash         string
	ChatID           int64
	TelegramUsername string
	TelegramName     string
	BotUsername      string
	LinkedAt         time.Time
}

// TelegramLinkRepository guarda el vínculo de cada usuario con su Telegram
// (una fila por usuario).
type TelegramLinkRepository struct {
	db Querier
}

func NewTelegramLinkRepository(db Querier) *TelegramLinkRepository {
	return &TelegramLinkRepository{db: db}
}

const telegramLinkSelect = `
	SELECT l.id, l.user_id, u.username, l.status, l.notifications_enabled, l.chat_id,
	       l.telegram_username, l.telegram_name, l.bot_username, l.link_code_expires_at, l.linked_at
	FROM ecom_user_telegram_links l
	JOIN ecom_api_user u ON u.id = l.user_id
	WHERE l.deleted_at IS NULL AND `

// findOne devuelve (nil, nil) cuando no hay fila.
func (r *TelegramLinkRepository) findOne(ctx context.Context, condition string, arg any) (*TelegramLinkDTO, error) {
	var (
		dto                                 TelegramLinkDTO
		chatID                              sql.NullInt64
		telegramUsername, name, botUsername sql.NullString
		expiresAt, linkedAt                 sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, telegramLinkSelect+condition, arg).Scan(
		&dto.ID, &dto.UserID, &dto.Username, &dto.Status, &dto.NotificationsEnabled, &chatID,
		&telegramUsername, &name, &botUsername, &expiresAt, &linkedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading telegram link: %w", err)
	}

	if chatID.Valid {
		dto.ChatID = &chatID.Int64
	}
	if expiresAt.Valid {
		dto.LinkCodeExpiresAt = &expiresAt.Time
	}
	if linkedAt.Valid {
		dto.LinkedAt = &linkedAt.Time
	}
	dto.TelegramUsername = telegramUsername.String
	dto.TelegramName = name.String
	dto.BotUsername = botUsername.String
	return &dto, nil
}

func (r *TelegramLinkRepository) FindByUserID(ctx context.Context, userID int64) (*TelegramLinkDTO, error) {
	return r.findOne(ctx, "l.user_id = ?", userID)
}

func (r *TelegramLinkRepository) FindByChatID(ctx context.Context, chatID int64) (*TelegramLinkDTO, error) {
	return r.findOne(ctx, "l.chat_id = ?", chatID)
}

// FindByCodeHash solo encuentra códigos de usuarios activos. La caducidad la
// revisa quien llama.
func (r *TelegramLinkRepository) FindByCodeHash(ctx context.Context, codeHash string) (*TelegramLinkDTO, error) {
	return r.findOne(ctx, "u.is_active = 1 AND l.link_code_hash = ?", codeHash)
}

// SaveLinkCode crea la fila del usuario o le pone un código nuevo. No cambia el
// estado de un vínculo existente: sigue vigente hasta que el código se use.
func (r *TelegramLinkRepository) SaveLinkCode(ctx context.Context, userID int64, codeHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_user_telegram_links (user_id, status, link_code_hash, link_code_expires_at, created_by)
		VALUES (?, 'pending', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			link_code_hash = VALUES(link_code_hash),
			link_code_expires_at = VALUES(link_code_expires_at),
			updated_by = VALUES(created_by),
			deleted_at = NULL
	`, userID, codeHash, expiresAt, userID)
	if err != nil {
		return fmt.Errorf("saving telegram link code: %w", err)
	}
	return nil
}

// Complete consume el código (un solo uso). Devuelve false si ya no estaba.
func (r *TelegramLinkRepository) Complete(ctx context.Context, input CompleteTelegramLinkInput) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ecom_user_telegram_links
		SET status = 'linked', chat_id = ?, telegram_username = ?, telegram_name = ?, bot_username = ?,
		    linked_at = ?, link_code_hash = NULL, link_code_expires_at = NULL, updated_by = user_id
		WHERE id = ? AND link_code_hash = ? AND deleted_at IS NULL
	`,
		input.ChatID, nullableString(input.TelegramUsername), nullableString(input.TelegramName),
		input.BotUsername, input.LinkedAt, input.ID, input.CodeHash,
	)
	if err != nil {
		return false, fmt.Errorf("completing telegram link: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("completing telegram link: %w", err)
	}
	return affected > 0, nil
}

func (r *TelegramLinkRepository) Unlink(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_user_telegram_links
		SET status = 'unlinked', chat_id = NULL, telegram_username = NULL, telegram_name = NULL,
		    bot_username = NULL, linked_at = NULL, link_code_hash = NULL, link_code_expires_at = NULL,
		    updated_by = ?
		WHERE user_id = ? AND deleted_at IS NULL
	`, userID, userID)
	if err != nil {
		return fmt.Errorf("unlinking telegram: %w", err)
	}
	return nil
}

func (r *TelegramLinkRepository) SetNotificationsEnabled(ctx context.Context, userID int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_user_telegram_links
		SET notifications_enabled = ?, updated_by = ?
		WHERE user_id = ? AND deleted_at IS NULL
	`, enabled, userID, userID)
	if err != nil {
		return fmt.Errorf("updating telegram notifications switch: %w", err)
	}
	return nil
}

// SetBlockedByChatID pasa el vínculo del chat de 'linked' a 'blocked' o al
// revés, según el usuario haya bloqueado o desbloqueado al bot.
func (r *TelegramLinkRepository) SetBlockedByChatID(ctx context.Context, chatID int64, blocked bool) error {
	from, to := TelegramLinkLinked, TelegramLinkBlocked
	if !blocked {
		from, to = TelegramLinkBlocked, TelegramLinkLinked
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_user_telegram_links
		SET status = ?, updated_by = NULL
		WHERE chat_id = ? AND status = ? AND deleted_at IS NULL
	`, to, chatID, from)
	if err != nil {
		return fmt.Errorf("updating blocked telegram link: %w", err)
	}
	return nil
}

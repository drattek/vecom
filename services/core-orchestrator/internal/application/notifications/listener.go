package notifications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/infrastructure/telegram"
)

const (
	pollWait = 25 * time.Second
	// idleDelay: espera mientras las notificaciones están apagadas.
	idleDelay = 10 * time.Second
	// errorDelay: espera tras un fallo, para no saturar Telegram ni el log.
	errorDelay = 30 * time.Second
)

// Listen recibe los mensajes que le llegan al bot (getUpdates) para completar
// vinculaciones y detectar bloqueos. Bloquea hasta que ctx se cancela: correr
// con `go safe.Supervise(...)`.
//
// Solo puede haber un proceso consultando cada bot: cada entorno usa el suyo.
func (s *Service) Listen(ctx context.Context) {
	lastError := ""
	for ctx.Err() == nil {
		delay, err := s.pollOnce(ctx)
		switch {
		case err == nil:
			lastError = ""
		case ctx.Err() != nil:
			return
		case err.Error() != lastError:
			// Un fallo persistente se registra una sola vez.
			lastError = err.Error()
			log.Printf("telegram listener: %v", err)
		}

		if delay <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// pollOnce hace una consulta y devuelve cuánto esperar antes de la siguiente.
func (s *Service) pollOnce(ctx context.Context) (time.Duration, error) {
	bot, ok, err := s.activeBot(ctx)
	if err != nil {
		return errorDelay, err
	}
	if !ok {
		return idleDelay, nil
	}

	offset := bot.updateOffset
	if offset > 0 {
		offset++
	}
	updates, err := s.bot.GetUpdates(ctx, bot.token, offset, pollWait)
	if err != nil {
		var apiErr *telegram.APIError
		if errors.As(err, &apiErr) {
			if apiErr.RetryAfter > 0 {
				return apiErr.RetryAfter, err
			}
			if apiErr.Code == 409 {
				return errorDelay, fmt.Errorf("otro proceso está usando este bot, o tiene un webhook configurado: %w", err)
			}
		}
		return errorDelay, err
	}

	processed := bot.updateOffset
	var handleErr error
	for _, update := range updates {
		// Si falla (p. ej. la base de datos), no se avanza: se reintenta.
		if handleErr = s.handleUpdate(ctx, bot, update); handleErr != nil {
			break
		}
		processed = update.UpdateID
	}

	if processed != bot.updateOffset {
		if err := s.settings.SaveUpdateOffset(ctx, bot.username, processed); err != nil {
			return errorDelay, err
		}
	}
	if handleErr != nil {
		return errorDelay, handleErr
	}
	return 0, nil
}

func (s *Service) handleUpdate(ctx context.Context, bot activeBot, update telegram.Update) error {
	if member := update.MyChatMember; member != nil && member.Chat.Type == "private" {
		switch member.NewChatMember.Status {
		case "kicked":
			return s.links.SetBlockedByChatID(ctx, member.Chat.ID, true)
		case "member":
			return s.links.SetBlockedByChatID(ctx, member.Chat.ID, false)
		}
		return nil
	}

	message := update.Message
	if message == nil || message.Chat.Type != "private" {
		return nil
	}
	fields := strings.Fields(message.Text)
	if len(fields) == 0 || fields[0] != "/start" {
		return nil
	}
	if len(fields) < 2 {
		s.reply(ctx, bot, message.Chat.ID, "Para recibir avisos, entra al dashboard y usa \"Vincular Telegram\" en Mi perfil.")
		return nil
	}
	return s.completeLink(ctx, bot, message, fields[1])
}

func (s *Service) completeLink(ctx context.Context, bot activeBot, message *telegram.Message, code string) error {
	const invalidCode = "Este enlace no es válido o ya venció. Genera uno nuevo desde Mi perfil en el dashboard."

	codeHash := hashLinkCode(code)
	link, err := s.links.FindByCodeHash(ctx, codeHash)
	if err != nil {
		return err
	}
	if link == nil || link.LinkCodeExpiresAt == nil || s.now().After(*link.LinkCodeExpiresAt) {
		s.reply(ctx, bot, message.Chat.ID, invalidCode)
		return nil
	}

	// No se le quita el chat a otro usuario: con un enlace ajeno se podrían
	// desviar los avisos de alguien.
	owner, err := s.links.FindByChatID(ctx, message.Chat.ID)
	if err != nil {
		return err
	}
	if owner != nil && owner.UserID != link.UserID {
		s.reply(ctx, bot, message.Chat.ID, "Este Telegram ya está vinculado al usuario "+owner.Username+". Desvincúlalo desde su perfil antes de usarlo con otro.")
		return nil
	}

	input := mysqlInfra.CompleteTelegramLinkInput{
		ID:          link.ID,
		CodeHash:    codeHash,
		ChatID:      message.Chat.ID,
		BotUsername: bot.username,
		LinkedAt:    s.now(),
	}
	if from := message.From; from != nil {
		input.TelegramUsername = from.Username
		input.TelegramName = strings.TrimSpace(from.FirstName + " " + from.LastName)
	}

	completed, err := s.links.Complete(ctx, input)
	if err != nil {
		return err
	}
	if !completed {
		s.reply(ctx, bot, message.Chat.ID, invalidCode)
		return nil
	}

	s.reply(ctx, bot, message.Chat.ID, "✅ Cuenta vinculada al usuario "+link.Username+" de Vegusa. Elige tus avisos en Mi perfil.")
	return nil
}

// reply no devuelve error: si la respuesta falla, el update ya quedó atendido.
func (s *Service) reply(ctx context.Context, bot activeBot, chatID int64, text string) {
	if err := s.bot.SendMessage(ctx, bot.token, chatID, text); err != nil {
		log.Printf("telegram listener: could not reply: %v", err)
	}
}

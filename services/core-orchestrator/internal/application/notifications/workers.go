package notifications

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/infrastructure/telegram"
	"core-orchestrator/internal/shared/safe"
)

const (
	// monitorTick es cada cuánto se revisa qué comprobaciones tocan.
	monitorTick = time.Minute
	// erpSilenceEvery es cada cuánto se revisa la falta de sincronización.
	erpSilenceEvery = 15 * time.Minute

	// dispatchIdle: espera cuando no hay nada que enviar.
	dispatchIdle = 5 * time.Second
	// sendSpacing respeta el límite de Telegram de un mensaje por segundo a un
	// mismo chat.
	sendSpacing = 1100 * time.Millisecond
	sendLock    = 2 * time.Minute
	maxAttempts = 6
	// staleAfter: lo que no se pudo enviar en este plazo ya no se envía.
	staleAfter = 24 * time.Hour
)

var retryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour}

// Monitor ejecuta las comprobaciones periódicas: conexiones, silencio del ERP,
// errores de publicación y lotes terminados. Solo trabaja con las notificaciones activadas. Bloquea
// hasta que ctx se cancela: correr con `go safe.Supervise(...)`.
func (s *Service) Monitor(ctx context.Context) {
	ticker := time.NewTicker(monitorTick)
	defer ticker.Stop()

	var lastSilenceCheck, lastPublicationCheck time.Time
	lastErrors := map[string]string{}
	run := func(name string, check func(context.Context) error) {
		safe.Do("notifications "+name, func() {
			err := check(ctx)
			if err == nil || ctx.Err() != nil {
				delete(lastErrors, name)
				return
			}
			// Un fallo persistente se registra una sola vez.
			if lastErrors[name] != err.Error() {
				lastErrors[name] = err.Error()
				log.Printf("notifications monitor: %s: %v", name, err)
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		_, active, err := s.activeBot(ctx)
		if err != nil || !active {
			continue
		}
		run("finished batches", s.checkFinishedBatches)
		if s.now().Sub(lastPublicationCheck) >= publicationCheckEvery {
			lastPublicationCheck = s.now()
			run("publication errors", s.checkPublicationErrors)
		}
		if s.now().Sub(lastSilenceCheck) >= erpSilenceEvery {
			lastSilenceCheck = s.now()
			run("erp silence", s.checkErpSilence)
		}
		// Al final: es lo único que depende de servicios externos.
		run("connections", s.checkConnections)
	}
}

type deliveryStore interface {
	ExpireStale(ctx context.Context, maxAge time.Duration) error
	MarkSent(ctx context.Context, id int64, sentAt time.Time) error
	Retry(ctx context.Context, id int64, nextAttempt time.Time, reason string) error
	Close(ctx context.Context, id int64, status, reason string) error
}

type deliveryClaimer func(ctx context.Context, now time.Time) (*mysqlInfra.NotificationDeliveryDTO, error)

func newSQLDeliveryClaimer(db *sql.DB) deliveryClaimer {
	return func(ctx context.Context, now time.Time) (*mysqlInfra.NotificationDeliveryDTO, error) {
		var claimed *mysqlInfra.NotificationDeliveryDTO
		err := mysqlInfra.WithinTx(ctx, db, func(tx *sql.Tx) error {
			var err error
			claimed, err = mysqlInfra.NewNotificationDeliveryRepository(tx).ClaimNext(ctx, now, sendLock)
			return err
		})
		return claimed, err
	}
}

// Dispatch envía lo que hay en la cola, de uno en uno. Bloquea hasta que ctx se
// cancela: correr con `go safe.Supervise(...)`.
func (s *Service) Dispatch(ctx context.Context) {
	lastError := ""
	for ctx.Err() == nil {
		delay, err := s.dispatchOnce(ctx)
		switch {
		case err == nil:
			lastError = ""
		case ctx.Err() != nil:
			return
		case err.Error() != lastError:
			lastError = err.Error()
			log.Printf("notifications dispatcher: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// dispatchOnce envía a lo sumo un aviso y devuelve cuánto esperar.
func (s *Service) dispatchOnce(ctx context.Context) (time.Duration, error) {
	bot, active, err := s.activeBot(ctx)
	if err != nil {
		return errorDelay, err
	}
	if !active {
		return idleDelay, nil
	}
	if err := s.deliveries.ExpireStale(ctx, staleAfter); err != nil {
		return errorDelay, err
	}

	delivery, err := s.claimDelivery(ctx, s.now())
	if err != nil {
		return errorDelay, err
	}
	if delivery == nil {
		return dispatchIdle, nil
	}

	// El resultado se guarda aunque el servicio se esté apagando: si no, el
	// aviso se repetiría.
	saveCtx, cancel := reportContext(ctx)
	defer cancel()

	// El destinatario pudo desvincularse o apagar los avisos desde que se
	// encoló.
	link, err := s.links.FindByUserID(ctx, delivery.UserID)
	if err != nil {
		return errorDelay, errors.Join(err, s.deliveries.Retry(saveCtx, delivery.ID, s.now().Add(time.Minute), "could not read the telegram link"))
	}
	if link == nil || link.ChatID == nil || link.Status != mysqlInfra.TelegramLinkLinked ||
		!link.NotificationsEnabled || link.BotUsername != bot.username {
		return sendSpacing, s.deliveries.Close(saveCtx, delivery.ID, "cancelled", "the user is no longer receiving notifications")
	}

	if len(delivery.AttachmentContent) > 0 {
		err = s.bot.SendDocument(ctx, bot.token, *link.ChatID, delivery.AttachmentName, delivery.AttachmentContent, delivery.Message)
	} else {
		err = s.bot.SendMessage(ctx, bot.token, *link.ChatID, delivery.Message)
	}
	if err == nil {
		return sendSpacing, s.deliveries.MarkSent(saveCtx, delivery.ID, s.now())
	}

	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.RetryAfter > 0:
			// Límite de frecuencia: se espera lo que pide Telegram.
			return apiErr.RetryAfter, s.deliveries.Retry(saveCtx, delivery.ID, s.now().Add(apiErr.RetryAfter), err.Error())
		case apiErr.Code == 403:
			return sendSpacing, errors.Join(
				s.links.SetBlockedByChatID(saveCtx, *link.ChatID, true),
				s.deliveries.Close(saveCtx, delivery.ID, "cancelled", "the user blocked the bot"),
			)
		case apiErr.Code == 400:
			return sendSpacing, errors.Join(err, s.deliveries.Close(saveCtx, delivery.ID, "failed", err.Error()))
		}
	}

	if delivery.Attempts >= maxAttempts {
		return errorDelay, errors.Join(err, s.deliveries.Close(saveCtx, delivery.ID, "failed", err.Error()))
	}
	delay := retryDelays[min(delivery.Attempts, len(retryDelays))-1]
	return sendSpacing, errors.Join(err, s.deliveries.Retry(saveCtx, delivery.ID, s.now().Add(delay), err.Error()))
}

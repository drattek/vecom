package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"core-orchestrator/internal/shared/safe"
)

const (
	jobStockPauseSummary = "stock_pause_summary"

	// jobLock: plazo tras el cual una corrida abandonada se puede retomar.
	jobLock = 2 * time.Hour
	// Espera antes de reintentar una corrida fallida. La de cierres es mayor
	// porque cada intento gasta cuota de la API de MercadoLibre.
	summaryRetryDelay = 10 * time.Minute
	closureRetryDelay = 30 * time.Minute
)

// clockTime es una hora del día.
type clockTime struct{ hour, minute int }

// closureCheckTimes son fijas (ADR 0012).
var closureCheckTimes = []clockTime{{0, 0}, {12, 0}}

type jobStore interface {
	Claim(ctx context.Context, jobCode string, scheduledFor, now time.Time, lockFor time.Duration) (bool, error)
	Complete(ctx context.Context, jobCode string, scheduledFor, finishedAt time.Time, windowFrom, windowTo *time.Time) error
	Fail(ctx context.Context, jobCode string, scheduledFor, retryAt time.Time, reason string) error
	IsDone(ctx context.Context, jobCode string, scheduledFor time.Time) (bool, error)
	LastWindowEnd(ctx context.Context, jobCode string) (*time.Time, error)
}

// latestSlot es el horario más reciente, en loc, que ya pasó. Si el servicio
// estuvo apagado, solo se ejecuta ese.
func latestSlot(now time.Time, loc *time.Location, times []clockTime) time.Time {
	local := now.In(loc)
	var latest time.Time
	for _, dayOffset := range []int{-1, 0} {
		for _, at := range times {
			slot := time.Date(local.Year(), local.Month(), local.Day()+dayOffset, at.hour, at.minute, 0, 0, loc)
			if !slot.After(local) && slot.After(latest) {
				latest = slot
			}
		}
	}
	return latest
}

// jobWindow es el periodo que cubrió una corrida, si aplica.
type jobWindow struct{ from, to *time.Time }

// runSlot ejecuta la tarea del horario si nadie lo ha hecho.
func (s *Service) runSlot(ctx context.Context, jobCode string, slot time.Time, retryDelay time.Duration, job func(context.Context) (jobWindow, error)) error {
	claimed, err := s.jobs.Claim(ctx, jobCode, slot, s.now(), jobLock)
	if err != nil || !claimed {
		return err
	}

	var (
		window jobWindow
		jobErr error
	)
	if !safe.Do("notifications job "+jobCode, func() { window, jobErr = job(ctx) }) {
		jobErr = fmt.Errorf("job %s panicked", jobCode)
	}

	// El resultado se guarda aunque el servicio se esté apagando.
	saveCtx, cancel := reportContext(ctx)
	defer cancel()
	if jobErr != nil {
		if err := s.jobs.Fail(saveCtx, jobCode, slot, s.now().Add(retryDelay), shortError(jobErr.Error())); err != nil {
			log.Printf("notifications: %v", err)
		}
		return jobErr
	}
	return s.jobs.Complete(saveCtx, jobCode, slot, s.now(), window.from, window.to)
}

// dueSlot es el horario más reciente que ya pasó, en la zona configurada. times
// se calcula con la configuración vigente.
func (s *Service) dueSlot(ctx context.Context, times func(dailySummaryTime string) ([]clockTime, error)) (time.Time, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading timezone %q: %w", settings.Timezone, err)
	}
	at, err := times(settings.DailySummaryTime)
	if err != nil {
		return time.Time{}, err
	}
	// La base guarda las fechas en la zona del proceso (loc=Local).
	return latestSlot(s.now(), location, at).In(time.Local), nil
}

func summaryTimes(dailySummaryTime string) ([]clockTime, error) {
	parsed, err := time.Parse("15:04", shortTime(dailySummaryTime))
	if err != nil {
		return nil, fmt.Errorf("invalid daily summary time %q: %w", dailySummaryTime, err)
	}
	return []clockTime{{parsed.Hour(), parsed.Minute()}}, nil
}

// runScheduled llama cada minuto a una tarea de horario fijo, que decide si le
// toca. Solo trabaja con las notificaciones activadas.
func (s *Service) runScheduled(ctx context.Context, name string, task func(context.Context) error) {
	ticker := time.NewTicker(monitorTick)
	defer ticker.Stop()

	lastError := ""
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
		err = task(ctx)
		switch {
		case err == nil:
			lastError = ""
		case ctx.Err() != nil:
			return
		case err.Error() != lastError:
			lastError = err.Error()
			log.Printf("notifications job %s: %v", name, err)
		}
	}
}

// ScheduleStockSummary envía el resumen diario de pausas por falta de stock a la
// hora configurada. Bloquea hasta que ctx se cancela.
func (s *Service) ScheduleStockSummary(ctx context.Context) {
	s.runScheduled(ctx, jobStockPauseSummary, func(ctx context.Context) error {
		slot, err := s.dueSlot(ctx, summaryTimes)
		if err != nil {
			return err
		}
		return s.runSlot(ctx, jobStockPauseSummary, slot, summaryRetryDelay, s.sendStockPauseSummary)
	})
}

// ScheduleClosureCheck revisa las publicaciones cerradas en MercadoLibre a las
// 0:00 y a las 12:00. Va aparte porque puede tardar varios minutos. Bloquea
// hasta que ctx se cancela.
func (s *Service) ScheduleClosureCheck(ctx context.Context) {
	s.runScheduled(ctx, "meli_closure_check", func(ctx context.Context) error {
		slot, err := s.dueSlot(ctx, func(string) ([]clockTime, error) { return closureCheckTimes, nil })
		if err != nil {
			return err
		}
		return s.checkListingClosures(ctx, slot)
	})
}

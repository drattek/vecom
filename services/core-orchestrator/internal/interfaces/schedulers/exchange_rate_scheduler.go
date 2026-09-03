package schedulers

import (
	"context"
	"log"
	"time"

	pricingApp "core-orchestrator/internal/application/pricing"
	"core-orchestrator/internal/shared/safe"
)

type ExchangeRateSchedulerConfig struct {
	// RunAtHour/RunAtMinute is the server-local time of day (24h clock) at
	// which the scheduler runs once per day. Default is 08:00 server time,
	// which on the UTC deployment server is 02:00 CDMX (UTC-6). Banxico
	// publishes the FIX rate (series SF43718) ~12:00-13:00 hrs CDMX, so an
	// early-morning run always consumes the latest rate already published
	// the prior business day.
	RunAtHour   int
	RunAtMinute int
}

// ExchangeRateScheduler calls SIEExchangeRateUpdater.UpdateUSDToMXN once a
// day, keeping ecom_exchange_rates' USD→MXN row current with Banco de
// México's published FIX rate — the rate EffectivePriceResolver applies
// whenever a product's effective price comes from a USD-denominated price
// list but the marketplace sync target (MercadoLibre, Odoo) needs MXN.
type ExchangeRateScheduler struct {
	updater *pricingApp.SIEExchangeRateUpdater
	config  ExchangeRateSchedulerConfig
}

func NewExchangeRateScheduler(updater *pricingApp.SIEExchangeRateUpdater, config ExchangeRateSchedulerConfig) *ExchangeRateScheduler {
	return &ExchangeRateScheduler{updater: updater, config: config}
}

// Start waits until the next occurrence of config.RunAtHour:RunAtMinute
// (server-local time; see nextRunTime in compatibilities_fix_scheduler.go
// for the same DST-safe recompute-each-iteration approach), runs once, then
// repeats for the following day — until ctx is cancelled. It is blocking —
// call it with `go scheduler.Start(ctx)`.
func (s *ExchangeRateScheduler) Start(ctx context.Context) {
	for {
		next := nextRunTime(time.Now(), s.config.RunAtHour, s.config.RunAtMinute)
		log.Printf("exchange rate scheduler: next run scheduled for %s", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			// Barrera de panic por corrida: si UpdateUSDToMXN panic-ea, se
			// saltea la corrida de hoy y el scheduler sigue programando la de
			// mañana, en vez de que safe.Supervise reinicie todo el loop.
			safe.Do("exchange rate scheduler run", func() { s.runOnce(ctx) })
		}
	}
}

func (s *ExchangeRateScheduler) runOnce(ctx context.Context) {
	rate, asOf, err := s.updater.UpdateUSDToMXN(ctx)
	if err != nil {
		log.Printf("exchange rate scheduler: error updating USD/MXN rate: %v", err)
		return
	}

	log.Printf("exchange rate scheduler: USD/MXN rate updated to %.4f (Banxico value as of %s)", rate, asOf.Format("2006-01-02"))
}

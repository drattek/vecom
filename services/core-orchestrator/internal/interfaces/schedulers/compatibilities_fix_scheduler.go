package schedulers

import (
	"context"
	"fmt"
	"log"
	"time"

	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

// compatibilitiesFixChannelCode is the only channel this scheduler ever
// targets — vehicle compatibility reporting (and the incomplete_compatibilities
// tag it clears) is a MercadoLibre-specific concept, already enforced by
// MercadoLibreCompatibilityService itself (see ErrNotMercadoLibreConnection),
// so unlike TokenRefreshScheduler this has no per-channel Register: there is
// only ever one channel to poll.
const compatibilitiesFixChannelCode = "MERCADOLIBRE"

// CompatibilitiesFixer is implemented by *sync.MercadoLibreCompatibilityService
// (duck-typed against its existing FixUnderReviewListings signature — no
// adapter needed to register it) — kept as a narrow interface here so this
// package depends on the one method it actually calls.
type CompatibilitiesFixer interface {
	FixUnderReviewListings(ctx context.Context, input syncApp.FixCompatibilitiesInput) (*syncApp.FixCompatibilitiesResult, error)
}

type CompatibilitiesFixSchedulerConfig struct {
	// RunAtHour/RunAtMinute is the server-local time of day (24h clock) at
	// which the scheduler runs once per day — e.g. 12/0 for noon.
	RunAtHour   int
	RunAtMinute int
}

// CompatibilitiesFixScheduler calls FixUnderReviewListings once a day for
// every active MERCADOLIBRE connection, so a listing that needs vehicle
// compatibilities reported gets fixed automatically instead of depending on
// someone calling POST /api/marketplaces/mercadolibre/compatibilities/fix-under-review
// by hand. It reuses FixUnderReviewListings exactly as-is: that method
// already re-checks each item's real incomplete_compatibilities tag against
// MercadoLibre before pushing anything, so a run that finds nothing to fix
// costs reads, not writes. FixUnderReviewListings processes every eligible
// row in one call (no per-call cap), so a single daily run works through the
// full backlog for a connection rather than only its first few rows.
type CompatibilitiesFixScheduler struct {
	connectionRepository *mysqlInfra.ChannelConnectionRepository
	fixer                CompatibilitiesFixer
	config               CompatibilitiesFixSchedulerConfig
}

func NewCompatibilitiesFixScheduler(
	connectionRepository *mysqlInfra.ChannelConnectionRepository,
	fixer CompatibilitiesFixer,
	config CompatibilitiesFixSchedulerConfig,
) *CompatibilitiesFixScheduler {
	return &CompatibilitiesFixScheduler{
		connectionRepository: connectionRepository,
		fixer:                fixer,
		config:               config,
	}
}

// Start waits until the next occurrence of config.RunAtHour:RunAtMinute
// (server-local time), runs once, then repeats for the following day — until
// ctx is cancelled. It is blocking — call it with `go scheduler.Start(ctx)`.
// Unlike a fixed-interval ticker, recomputing the next run time on every
// iteration (rather than a flat 24h ticker) keeps it locked to the same
// wall-clock time across DST transitions.
func (s *CompatibilitiesFixScheduler) Start(ctx context.Context) {
	for {
		next := nextRunTime(time.Now(), s.config.RunAtHour, s.config.RunAtMinute)
		log.Printf("compatibilities fix scheduler: next run scheduled for %s", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			// Barrera de panic por corrida: un panic fuera del loop por
			// conexión (p. ej. listando conexiones activas) saltea la corrida de
			// hoy en vez de que safe.Supervise reinicie todo el loop.
			safe.Do("compatibilities fix scheduler run", func() { s.runOnce(ctx) })
		}
	}
}

// nextRunTime returns the next time of day matching hour:minute strictly
// after now — today if that time hasn't passed yet, tomorrow otherwise.
func nextRunTime(now time.Time, hour, minute int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func (s *CompatibilitiesFixScheduler) runOnce(ctx context.Context) {
	connections, err := s.connectionRepository.FindActiveByChannelCode(ctx, compatibilitiesFixChannelCode)
	if err != nil {
		log.Printf("compatibilities fix scheduler: error listing active %s connections: %v", compatibilitiesFixChannelCode, err)
		return
	}

	if len(connections) == 0 {
		return
	}

	log.Printf("compatibilities fix scheduler: poll found %d active %s connection(s)", len(connections), compatibilitiesFixChannelCode)

	for _, connection := range connections {
		connection := connection

		// Barrera de panic por conexión: un panic procesando una no debe
		// abortar las demás ni tumbar el scheduler.
		safe.Do(fmt.Sprintf("compatibilities fix scheduler: connection %d", connection.ID), func() {
			result, err := s.fixer.FixUnderReviewListings(ctx, syncApp.FixCompatibilitiesInput{
				ConnectionID: connection.ID,
			})
			if err != nil {
				log.Printf("compatibilities fix scheduler: connection %d — error: %v", connection.ID, err)
				return
			}

			if len(result.Results) == 0 {
				return
			}

			fixed, skipped, failed := 0, 0, 0
			for _, outcome := range result.Results {
				switch {
				case outcome.Success:
					fixed++
				case outcome.Skipped:
					skipped++
				default:
					failed++
				}
			}

			log.Printf("compatibilities fix scheduler: connection %d — processed %d listing(s): %d fixed, %d skipped, %d failed", connection.ID, len(result.Results), fixed, skipped, failed)
		})
	}
}

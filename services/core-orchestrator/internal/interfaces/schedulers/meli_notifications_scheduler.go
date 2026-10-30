package schedulers

import (
	"context"
	"log"
	"time"

	meliNotificationsApp "core-orchestrator/internal/application/meli_notifications"
)

// MeliNotificationsSchedulerConfig bundles the timing + feature flag the
// shadow processor is wired with.
type MeliNotificationsSchedulerConfig struct {
	// Enabled gates the whole scheduler. When false, Start returns
	// immediately after a one-line log so the deploy stays obvious.
	Enabled bool
	// PollInterval is how often RunOnce fires. 30s is a reasonable floor:
	// each run only queries ecom_meli_notifications WHERE status='pending'
	// (indexed) and the batch size caps how much work it does per tick.
	PollInterval time.Duration
}

// MeliNotificationsScheduler is a thin wrapper that ticks the shadow
// processor on a fixed interval. Siblings: TokenRefreshScheduler,
// ListingDiscoveryScheduler. Kept intentionally dumb — any processing
// cleverness (dead-letter, backoff beyond MarkFailed) lives in the
// processor itself.
type MeliNotificationsScheduler struct {
	processor *meliNotificationsApp.Processor
	config    MeliNotificationsSchedulerConfig
}

func NewMeliNotificationsScheduler(processor *meliNotificationsApp.Processor, config MeliNotificationsSchedulerConfig) *MeliNotificationsScheduler {
	if config.PollInterval <= 0 {
		config.PollInterval = 30 * time.Second
	}
	return &MeliNotificationsScheduler{processor: processor, config: config}
}

// Start runs the polling loop until ctx is cancelled. Blocking — launch
// with `go safe.Supervise(..., scheduler.Start)` from main.
func (s *MeliNotificationsScheduler) Start(ctx context.Context) {
	if !s.config.Enabled {
		log.Printf("meli notifications processor: scheduler disabled (MELI_NOTIFICATIONS_PROCESSOR_ENABLED!=true), not starting")
		return
	}

	log.Printf("meli notifications processor: scheduler started, interval=%s", s.config.PollInterval)

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	// First tick immediately so a cold start that already has backlog
	// doesn't wait a full interval before draining.
	s.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("meli notifications processor: scheduler stopping: %v", ctx.Err())
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *MeliNotificationsScheduler) runOnce(ctx context.Context) {
	if err := s.processor.RunOnce(ctx); err != nil {
		log.Printf("meli notifications processor: run failed: %v", err)
	}
}

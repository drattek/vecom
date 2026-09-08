package schedulers

import (
	"context"
	"log"
	"time"

	listingDiscoveryApp "core-orchestrator/internal/application/listing_discovery"
	"core-orchestrator/internal/shared/safe"
)

// ListingDiscoverySchedulerConfig sets the time of day the scan runs.
type ListingDiscoverySchedulerConfig struct {
	// RunAtHour/RunAtMinute is the server-local time of day (24h clock) the
	// discovery scan runs once per day. Same convention as the other
	// schedulers (see nextRunTime in compatibilities_fix_scheduler.go);
	// default is 01:00.
	RunAtHour   int
	RunAtMinute int
}

// ListingDiscoveryScheduler runs listing_discovery.Service.DiscoverAndEnqueue
// once a day: it scans every product that is ready to publish (stock, price,
// cover image, category, brand) and enqueues a 'listing' row in
// ecom_channel_sync_queue for each connection its category is mapped to that
// has no publication yet. The marketplace consumer (internal/workers) does
// the actual publishing. It is the only producer of ecom_channel_sync_queue.
type ListingDiscoveryScheduler struct {
	service *listingDiscoveryApp.Service
	config  ListingDiscoverySchedulerConfig
}

func NewListingDiscoveryScheduler(service *listingDiscoveryApp.Service, config ListingDiscoverySchedulerConfig) *ListingDiscoveryScheduler {
	return &ListingDiscoveryScheduler{service: service, config: config}
}

// Start waits until the next occurrence of config.RunAtHour:RunAtMinute
// (server-local time; same DST-safe recompute-each-iteration approach as
// nextRunTime in compatibilities_fix_scheduler.go), runs once, then repeats
// for the following day — until ctx is cancelled. It is blocking — call it
// with `go scheduler.Start(ctx)`.
func (s *ListingDiscoveryScheduler) Start(ctx context.Context) {
	for {
		next := nextRunTime(time.Now(), s.config.RunAtHour, s.config.RunAtMinute)
		log.Printf("listing discovery scheduler: next run scheduled for %s", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			// Barrera de panic por corrida: si DiscoverAndEnqueue panic-ea, se
			// saltea la corrida de hoy y el scheduler sigue programando la de
			// mañana, en vez de que safe.Supervise reinicie todo el loop.
			safe.Do("listing discovery scheduler run", func() { s.runOnce(ctx) })
		}
	}
}

func (s *ListingDiscoveryScheduler) runOnce(ctx context.Context) {
	result, err := s.service.DiscoverAndEnqueue(ctx)
	if err != nil {
		log.Printf("listing discovery scheduler: run failed: %v", err)
		return
	}

	log.Printf("listing discovery scheduler: run complete — scanned %d product(s), enqueued %d, reactivated %d, skipped %d",
		result.ScannedProducts, result.Enqueued, result.Reactivated, result.Skipped)
}

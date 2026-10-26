package schedulers

import (
	"context"
	"log"
	"time"

	meliBulkImportApp "core-orchestrator/internal/application/meli_bulk_import"
	"core-orchestrator/internal/shared/safe"
)

// MeliImportMissingSchedulerConfig sets the time of day the retry runs.
type MeliImportMissingSchedulerConfig struct {
	// RunAtHour/RunAtMinute is the server-local time of day (24h clock) the
	// retry runs once per day; default is 02:00.
	RunAtHour   int
	RunAtMinute int
}

// MeliImportMissingScheduler runs meli_bulk_import.Service.RetryMissing once a
// day: rows of "Productos → Importar" that were recorded as missing because
// their SKU wasn't in ecom_products are requeued as soon as the product exists,
// so the bulk import worker links the listing and pushes price/stock without
// anyone pressing "Reprocesar faltantes". Rows whose SKU still doesn't exist
// are left untouched.
type MeliImportMissingScheduler struct {
	service *meliBulkImportApp.Service
	config  MeliImportMissingSchedulerConfig
}

func NewMeliImportMissingScheduler(service *meliBulkImportApp.Service, config MeliImportMissingSchedulerConfig) *MeliImportMissingScheduler {
	return &MeliImportMissingScheduler{service: service, config: config}
}

// Start is blocking — call it with `go scheduler.Start(ctx)` — and follows the
// same wait-run-repeat loop as ListingDiscoveryScheduler.
func (s *MeliImportMissingScheduler) Start(ctx context.Context) {
	for {
		next := nextRunTime(time.Now(), s.config.RunAtHour, s.config.RunAtMinute)
		log.Printf("meli import missing scheduler: next run scheduled for %s", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			safe.Do("meli import missing scheduler run", func() { s.runOnce(ctx) })
		}
	}
}

func (s *MeliImportMissingScheduler) runOnce(ctx context.Context) {
	requeued, missing, err := s.service.RetryMissing(ctx)
	if err != nil {
		log.Printf("meli import missing scheduler: run failed after requeueing %d of %d missing row(s): %v", requeued, missing, err)
		return
	}
	log.Printf("meli import missing scheduler: run complete — %d missing row(s), %d requeued", missing, requeued)
}

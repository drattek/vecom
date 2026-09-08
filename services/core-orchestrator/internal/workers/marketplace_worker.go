// Package workers holds background jobs that push local state out to
// external systems (as opposed to internal/interfaces/consumers, which
// pulls ERP state in via RabbitMQ).
package workers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

// systemSyncWorkerActorID is recorded as updated_by for ecom_channel_sync_queue
// rows this worker mutates, since there is no authenticated user behind an
// automated poll (mirrors product_image_import's systemImportUserID).
const systemSyncWorkerActorID int64 = 1

const defaultMarketplaceWorkerBatchSize = 10

// ErrSyncNotReady signals a precondition that isn't met yet (missing image,
// missing category mapping, ...). Marketplace publishers wrap it (fmt.Errorf
// with %w) so errors.Is still matches through additional context; the worker
// itself no longer special-cases it (any publish error marks the row failed),
// but sync_odoo_products.go still returns it.
var ErrSyncNotReady = errors.New("product not ready to sync")

// QueuedListingPublisher is implemented by channel_listings.Service.
// PublishQueuedProduct re-attempts publishing every listing for productID on
// connectionID, recomputing the stock precondition and vehicle compatibility
// fan-out live (never off a snapshot taken when the entry was first queued).
// It returns an error wrapping channelListingsApp.ErrChannelListingsNotReady
// when the product no longer has stock, so the caller releases the queue
// entry back to pending instead of marking it failed.
type QueuedListingPublisher interface {
	PublishQueuedProduct(ctx context.Context, productID, connectionID int64) (*channelListingsApp.CreateListingsResult, error)
}

type MarketplaceWorkerConfig struct {
	// PollInterval is how often the worker checks for pending queue entries.
	PollInterval time.Duration
	// BatchSize caps how many pending entries are claimed per poll.
	BatchSize int
}

// MarketplaceWorker polls ecom_channel_sync_queue for pending 'listing'
// entries — the rows ListingDiscoveryScheduler produces — claims each one
// atomically and publishes it through channel_listings. It is the only
// consumer of the queue; ListingDiscoveryScheduler is the only producer.
type MarketplaceWorker struct {
	queueRepository *mysqlInfra.ChannelSyncQueueRepository
	publisher       QueuedListingPublisher
	config          MarketplaceWorkerConfig
}

func NewMarketplaceWorker(
	queueRepository *mysqlInfra.ChannelSyncQueueRepository,
	publisher QueuedListingPublisher,
	config MarketplaceWorkerConfig,
) *MarketplaceWorker {
	if config.BatchSize <= 0 {
		config.BatchSize = defaultMarketplaceWorkerBatchSize
	}

	return &MarketplaceWorker{
		queueRepository: queueRepository,
		publisher:       publisher,
		config:          config,
	}
}

// Start runs the polling loop until ctx is cancelled. It is blocking — call
// it with `go worker.Start(ctx)`.
func (w *MarketplaceWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	w.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *MarketplaceWorker) runOnce(ctx context.Context) {
	entries, err := w.queueRepository.FindPending(ctx, w.config.BatchSize)
	if err != nil {
		log.Printf("marketplace worker: error listing pending queue entries: %v", err)
		return
	}

	for _, entry := range entries {
		entry := entry
		// Barrera de panic por fila: una entrada envenenada se loguea con stack
		// y se saltea, sin abortar el resto del batch ni el poll.
		safe.Do(fmt.Sprintf("marketplace worker: queue entry %d", entry.ID), func() {
			w.processEntry(ctx, entry)
		})
	}
}

func (w *MarketplaceWorker) processEntry(ctx context.Context, entry mysqlInfra.ChannelSyncQueueDTO) {
	claimed, err := w.queueRepository.Claim(ctx, entry.ID)
	if err != nil {
		log.Printf("marketplace worker: error claiming queue entry %d: %v", entry.ID, err)
		return
	}
	if !claimed {
		return
	}

	result, err := w.publisher.PublishQueuedProduct(ctx, entry.ProductID, entry.ConnectionID)
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrChannelListingsNotReady) || errors.Is(err, channelListingsApp.ErrNoPublisherForChannel) {
			w.release(ctx, entry.ID, err.Error())
			return
		}

		log.Printf("marketplace worker: error publishing queue entry %d (product %d, connection %d): %v", entry.ID, entry.ProductID, entry.ConnectionID, err)
		w.fail(ctx, entry.ID, err.Error())
		return
	}

	if summary := outcomeErrorSummary(result); summary != "" {
		w.fail(ctx, entry.ID, summary)
		return
	}

	if err := w.queueRepository.MarkDone(ctx, entry.ID, systemSyncWorkerActorID); err != nil {
		log.Printf("marketplace worker: error marking queue entry %d done: %v", entry.ID, err)
	}
}

// outcomeErrorSummary joins every non-empty ListingOutcome.Error in result
// (sku=... prefixed, so a partial failure across several vehicle-fitment
// listings is still legible), or "" if every outcome succeeded or was
// skipped as already-listed.
func outcomeErrorSummary(result *channelListingsApp.CreateListingsResult) string {
	messages := make([]string, 0, len(result.Results))
	for _, outcome := range result.Results {
		if outcome.Error == "" {
			continue
		}
		messages = append(messages, fmt.Sprintf("sku=%s: %s", outcome.SKU, outcome.Error))
	}
	return strings.Join(messages, "; ")
}

func (w *MarketplaceWorker) fail(ctx context.Context, id int64, message string) {
	if err := w.queueRepository.MarkFailed(ctx, id, message, systemSyncWorkerActorID); err != nil {
		log.Printf("marketplace worker: error marking queue entry %d failed: %v", id, err)
	}
}

func (w *MarketplaceWorker) release(ctx context.Context, id int64, message string) {
	if err := w.queueRepository.ReleasePending(ctx, id, message, systemSyncWorkerActorID); err != nil {
		log.Printf("marketplace worker: error releasing queue entry %d: %v", id, err)
	}
}

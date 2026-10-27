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

// QueuedCompatibilitiesSyncer is implemented by channel_listings.Service.
// SyncProductCompatibilities pushes only the product's compatibilities to its
// existing publications on the connection; a failure of one publication is
// reported in its outcome's Error, not as a call error.
type QueuedCompatibilitiesSyncer interface {
	SyncProductCompatibilities(ctx context.Context, input channelListingsApp.ResyncListingsInput) (*channelListingsApp.RefreshListingsResult, error)
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
// consumer of the queue; ListingDiscoveryScheduler is the producer of the
// 'listing' rows. It also drains the 'compatibilities' rows a connection's
// "Sincronizar compatibilidades" button produces, through compatibilitiesSyncer.
type MarketplaceWorker struct {
	queueRepository       *mysqlInfra.ChannelSyncQueueRepository
	publisher             QueuedListingPublisher
	compatibilitiesSyncer QueuedCompatibilitiesSyncer
	config                MarketplaceWorkerConfig
}

func NewMarketplaceWorker(
	queueRepository *mysqlInfra.ChannelSyncQueueRepository,
	publisher QueuedListingPublisher,
	compatibilitiesSyncer QueuedCompatibilitiesSyncer,
	config MarketplaceWorkerConfig,
) *MarketplaceWorker {
	if config.BatchSize <= 0 {
		config.BatchSize = defaultMarketplaceWorkerBatchSize
	}

	return &MarketplaceWorker{
		queueRepository:       queueRepository,
		publisher:             publisher,
		compatibilitiesSyncer: compatibilitiesSyncer,
		config:                config,
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
	w.runListings(ctx)
	w.drainCompatibilities(ctx)
}

// drainCompatibilities processes every pending 'compatibilities' entry, batch
// after batch, instead of one batch per poll: the button enqueues a whole
// connection at once and each entry is a single cheap call to the channel, so
// waiting one poll interval per BatchSize products would take days. It stops
// when nothing is pending, when ctx is cancelled, or when a batch makes no
// progress (e.g. the database is failing), so it can't spin.
func (w *MarketplaceWorker) drainCompatibilities(ctx context.Context) {
	for ctx.Err() == nil {
		entries, err := w.queueRepository.FindPendingCompatibilities(ctx, w.config.BatchSize)
		if err != nil {
			log.Printf("marketplace worker: error listing pending compatibilities entries: %v", err)
			return
		}
		if len(entries) == 0 {
			return
		}

		claimedAny := false
		for _, entry := range entries {
			entry := entry
			safe.Do(fmt.Sprintf("marketplace worker: compatibilities entry %d", entry.ID), func() {
				if w.processCompatibilitiesEntry(ctx, entry) {
					claimedAny = true
				}
			})
		}
		if !claimedAny {
			return
		}
	}
}

// processCompatibilitiesEntry claims entry and pushes the product's
// compatibilities. It reports whether the entry was claimed (and so left
// pending), as the progress signal drainCompatibilities needs. Any outcome
// error marks the row failed with the joined messages; an entry whose product
// has no publication left finishes as done with nothing to push.
func (w *MarketplaceWorker) processCompatibilitiesEntry(ctx context.Context, entry mysqlInfra.ChannelSyncQueueDTO) bool {
	claimed, err := w.queueRepository.Claim(ctx, entry.ID)
	if err != nil {
		log.Printf("marketplace worker: error claiming compatibilities entry %d: %v", entry.ID, err)
		return false
	}
	if !claimed {
		return false
	}

	result, err := w.compatibilitiesSyncer.SyncProductCompatibilities(ctx, channelListingsApp.ResyncListingsInput{
		ProductID:    entry.ProductID,
		ConnectionID: entry.ConnectionID,
	})
	if err != nil {
		log.Printf("marketplace worker: error syncing compatibilities of entry %d (product %d, connection %d): %v", entry.ID, entry.ProductID, entry.ConnectionID, err)
		w.fail(ctx, entry.ID, err.Error())
		return true
	}

	messages := make([]string, 0, len(result.Results))
	for _, outcome := range result.Results {
		if outcome.Error != "" {
			messages = append(messages, fmt.Sprintf("sku=%s: %s", outcome.SKU, outcome.Error))
		}
	}
	if len(messages) > 0 {
		w.fail(ctx, entry.ID, strings.Join(messages, "; "))
		return true
	}

	if err := w.queueRepository.MarkDone(ctx, entry.ID, systemSyncWorkerActorID); err != nil {
		log.Printf("marketplace worker: error marking compatibilities entry %d done: %v", entry.ID, err)
	}
	return true
}

func (w *MarketplaceWorker) runListings(ctx context.Context) {
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

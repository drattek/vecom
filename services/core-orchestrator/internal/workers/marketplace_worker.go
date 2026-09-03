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

// ProductSyncer is implemented by each marketplace-specific sync service
// (Odoo today; MercadoLibre, Amazon, etc. later). Sync pushes/updates a
// single product on the given connection and is responsible for its own
// create-vs-update decision.
//
// Returning an error wrapping ErrSyncNotReady signals a precondition that
// isn't met yet (missing image, missing category mapping, ...): the queue
// entry is left pending instead of marked failed, so it is retried on a
// later poll once the precondition is met.
type ProductSyncer interface {
	Sync(ctx context.Context, productID, connectionID int64) error
}

// ErrSyncNotReady is not returned directly; syncers wrap it (fmt.Errorf
// with %w) so errors.Is still matches through additional context.
var ErrSyncNotReady = errors.New("product not ready to sync")

// ChannelListingsRetrier is implemented by channel_listings.Service.
// RetryQueuedEntry re-attempts publishing every listing for productID on
// connectionID, recomputing the stock/cover-image preconditions and vehicle
// compatibility fan-out live (never off a snapshot taken when the entry was
// first queued). It returns an error wrapping
// channelListingsApp.ErrChannelListingsNotReady when a precondition still
// isn't met, so the caller leaves the queue entry pending instead of
// marking it done or failed.
type ChannelListingsRetrier interface {
	RetryQueuedEntry(ctx context.Context, productID, connectionID int64) (*channelListingsApp.CreateListingsResult, error)
}

type MarketplaceWorkerConfig struct {
	// PollInterval is how often the worker checks for pending queue entries.
	PollInterval time.Duration
	// BatchSize caps how many pending entries are claimed per poll.
	BatchSize int
}

// MarketplaceWorker polls ecom_channel_sync_queue for pending entries and
// processes each one of two ways, chosen by entry.SyncType:
//
//   - channelListingsApp.QueueSyncType ("listing"): retried through
//     channelListingsRetrier — these are rows channel_listings.Service
//     itself queued after a stock/cover-image precondition failed on a
//     publish request.
//   - anything else (the "price"/"stock"/"full" values the pre-existing,
//     channel-agnostic /api/channel-sync-queue endpoint uses): dispatched to
//     the ProductSyncer registered for the connection's channel code
//     (ecom_channels.code), same as before. Adding a new marketplace to this
//     path means registering a new syncer here — the polling/dispatch logic
//     never changes.
type MarketplaceWorker struct {
	queueRepository        *mysqlInfra.ChannelSyncQueueRepository
	connectionRepository   *mysqlInfra.ChannelConnectionRepository
	channelRepository      *mysqlInfra.ChannelRepository
	channelListingsRetrier ChannelListingsRetrier
	config                 MarketplaceWorkerConfig
	syncers                map[string]ProductSyncer
}

func NewMarketplaceWorker(
	queueRepository *mysqlInfra.ChannelSyncQueueRepository,
	connectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelListingsRetrier ChannelListingsRetrier,
	config MarketplaceWorkerConfig,
) *MarketplaceWorker {
	if config.BatchSize <= 0 {
		config.BatchSize = defaultMarketplaceWorkerBatchSize
	}

	return &MarketplaceWorker{
		queueRepository:        queueRepository,
		connectionRepository:   connectionRepository,
		channelRepository:      channelRepository,
		channelListingsRetrier: channelListingsRetrier,
		config:                 config,
		syncers:                make(map[string]ProductSyncer),
	}
}

// Register associates a ProductSyncer with a channel code
// (ecom_channels.code, matched case-insensitively). Queue entries whose
// connection resolves to a channel with no registered syncer are left
// pending, untouched, until one is registered for it.
func (w *MarketplaceWorker) Register(channelCode string, syncer ProductSyncer) {
	w.syncers[normalizeChannelCode(channelCode)] = syncer
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

	if entry.SyncType == channelListingsApp.QueueSyncType {
		w.processChannelListingsEntry(ctx, entry)
		return
	}

	connection, err := w.connectionRepository.FindByID(ctx, entry.ConnectionID)
	if err != nil {
		w.fail(ctx, entry.ID, fmt.Sprintf("error loading connection %d: %v", entry.ConnectionID, err))
		return
	}

	channel, err := w.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		w.fail(ctx, entry.ID, fmt.Sprintf("error loading channel %d: %v", connection.ChannelID, err))
		return
	}

	syncer, ok := w.syncers[normalizeChannelCode(channel.Code)]
	if !ok {
		w.release(ctx, entry.ID, fmt.Sprintf("no syncer registered for channel %q", channel.Code))
		return
	}

	if err := syncer.Sync(ctx, entry.ProductID, entry.ConnectionID); err != nil {
		if errors.Is(err, ErrSyncNotReady) {
			w.release(ctx, entry.ID, err.Error())
			return
		}

		log.Printf("marketplace worker: error syncing queue entry %d (product %d, connection %d): %v", entry.ID, entry.ProductID, entry.ConnectionID, err)
		w.fail(ctx, entry.ID, err.Error())
		return
	}

	if err := w.queueRepository.MarkDone(ctx, entry.ID, systemSyncWorkerActorID); err != nil {
		log.Printf("marketplace worker: error marking queue entry %d done: %v", entry.ID, err)
	}
}

// processChannelListingsEntry handles a queue row created by
// channel_listings.Service.queuePending (entry.SyncType ==
// channelListingsApp.QueueSyncType), retrying it through
// channelListingsRetrier instead of the per-channel ProductSyncer registry.
func (w *MarketplaceWorker) processChannelListingsEntry(ctx context.Context, entry mysqlInfra.ChannelSyncQueueDTO) {
	if w.channelListingsRetrier == nil {
		w.release(ctx, entry.ID, "no channel listings retrier configured")
		return
	}

	result, err := w.channelListingsRetrier.RetryQueuedEntry(ctx, entry.ProductID, entry.ConnectionID)
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrChannelListingsNotReady) || errors.Is(err, channelListingsApp.ErrNoPublisherForChannel) {
			w.release(ctx, entry.ID, err.Error())
			return
		}

		log.Printf("marketplace worker: error retrying channel listings queue entry %d (product %d, connection %d): %v", entry.ID, entry.ProductID, entry.ConnectionID, err)
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

func normalizeChannelCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

package channel_listings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// EnqueueResyncResult reports how many products the bulk resync queued: Enqueued
// are new queue rows, Requeued are rows from an earlier run (done or failed)
// sent back to pending. Products whose entry is still pending or processing are
// in neither.
type EnqueueResyncResult struct {
	Enqueued int64 `json:"enqueued"`
	Requeued int64 `json:"requeued"`
}

// EnqueueConnectionResync queues, for every product published on connectionID,
// a 'resync' entry that MarketplaceWorker later resolves through
// ResyncProductListings (price and stock are pushed even when unchanged). It
// fails with ErrNoFullRefresherForChannel when the connection's channel has no
// FullRefresher, so the queue never fills with rows nobody can process.
func (s *Service) EnqueueConnectionResync(ctx context.Context, connectionID, actorID int64) (*EnqueueResyncResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidChannelConnection
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidChannelConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", connectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}

	if _, ok := s.fullRefreshers[normalizeChannelCode(channel.Code)]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoFullRefresherForChannel, channel.Code)
	}

	result := &EnqueueResyncResult{}
	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		enqueued, requeued, err := mysqlInfra.NewChannelSyncQueueRepository(tx).EnqueueResyncForConnection(ctx, connectionID, actorID)
		if err != nil {
			return err
		}
		result.Enqueued, result.Requeued = enqueued, requeued
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error enqueueing resync for connection %d: %w", connectionID, err)
	}

	return result, nil
}

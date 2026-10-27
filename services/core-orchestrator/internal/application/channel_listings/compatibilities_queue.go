package channel_listings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// resolveCompatibilitiesSyncer loads connectionID and its channel and returns
// the channel's CompatibilitiesSyncer, or ErrInvalidChannelConnection /
// ErrNoCompatibilitiesSyncerForChannel.
func (s *Service) resolveCompatibilitiesSyncer(ctx context.Context, connectionID int64) (CompatibilitiesSyncer, error) {
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

	syncer, ok := s.fullRefreshers[normalizeChannelCode(channel.Code)].(CompatibilitiesSyncer)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoCompatibilitiesSyncerForChannel, channel.Code)
	}

	return syncer, nil
}

// EnqueueCompatibilitiesResult reports how many products the bulk compatibilities
// sync queued: Enqueued are new queue rows, Requeued are rows from an earlier run
// (done or failed) sent back to pending. Products whose entry is still pending or
// processing are in neither.
type EnqueueCompatibilitiesResult struct {
	Enqueued int64 `json:"enqueued"`
	Requeued int64 `json:"requeued"`
}

// EnqueueConnectionCompatibilities queues, for every product published on
// connectionID, a 'compatibilities' entry that MarketplaceWorker later resolves
// through SyncProductCompatibilities. It fails with
// ErrNoCompatibilitiesSyncerForChannel when the connection's channel can't sync
// compatibilities, so the queue never fills with rows nobody can process.
func (s *Service) EnqueueConnectionCompatibilities(ctx context.Context, connectionID, actorID int64) (*EnqueueCompatibilitiesResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidChannelConnection
	}

	if _, err := s.resolveCompatibilitiesSyncer(ctx, connectionID); err != nil {
		return nil, err
	}

	result := &EnqueueCompatibilitiesResult{}
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		enqueued, requeued, err := mysqlInfra.NewChannelSyncQueueRepository(tx).EnqueueCompatibilitiesForConnection(ctx, connectionID, actorID)
		if err != nil {
			return err
		}
		result.Enqueued, result.Requeued = enqueued, requeued
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error enqueueing compatibilities for connection %d: %w", connectionID, err)
	}

	return result, nil
}

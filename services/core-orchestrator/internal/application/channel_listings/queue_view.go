package channel_listings

import (
	"context"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	defaultQueueViewPageSize = 50
	maxQueueViewPageSize     = 200
)

// QueueViewInput selects which rows of a connection's sync queue to list. Only
// the summary is returned when SyncType is empty.
type QueueViewInput struct {
	ConnectionID int64
	SyncType     string
	Status       string
	Offset       int
	PageSize     int
}

// QueueViewResult is a connection's queue progress: Summary counts every
// sync_type by status; Entries/Total are the rows of the requested SyncType.
type QueueViewResult struct {
	Summary []mysqlInfra.ChannelSyncQueueSummaryDTO   `json:"summary"`
	Entries []mysqlInfra.ChannelSyncQueueViewEntryDTO `json:"entries"`
	Total   int64                                     `json:"total"`
}

// QueueView reports the progress of the background syncs queued for a
// connection (listing publication, compatibilities, resync).
func (s *Service) QueueView(ctx context.Context, input QueueViewInput) (*QueueViewResult, error) {
	if input.ConnectionID <= 0 {
		return nil, ErrInvalidChannelConnection
	}
	if input.PageSize <= 0 {
		input.PageSize = defaultQueueViewPageSize
	}
	if input.PageSize > maxQueueViewPageSize {
		input.PageSize = maxQueueViewPageSize
	}
	if input.Offset < 0 {
		input.Offset = 0
	}

	repo := mysqlInfra.NewChannelSyncQueueRepository(s.db)
	summary, err := repo.SummaryByConnection(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error summarizing sync queue of connection %d: %w", input.ConnectionID, err)
	}

	result := &QueueViewResult{Summary: summary, Entries: []mysqlInfra.ChannelSyncQueueViewEntryDTO{}}
	if input.SyncType == "" {
		return result, nil
	}

	result.Entries, result.Total, err = repo.ListByConnection(ctx, input.ConnectionID, input.SyncType, input.Status, input.Offset, input.PageSize)
	if err != nil {
		return nil, fmt.Errorf("error listing sync queue of connection %d: %w", input.ConnectionID, err)
	}
	return result, nil
}

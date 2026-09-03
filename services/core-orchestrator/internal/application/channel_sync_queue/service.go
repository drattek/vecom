package channel_sync_queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrEmptyEnqueueRequest = errors.New("at least one item is required")

// EnqueueItemInput is one {sku, connectionIds} entry from the request: the
// product to sync, and every marketplace connection it should be queued
// for.
type EnqueueItemInput struct {
	SKU           string
	ConnectionIDs []int64
}

type EnqueueInput struct {
	Items     []EnqueueItemInput
	UpdatedBy int64
}

// EnqueueResultItem reports the outcome for a single (sku, connectionId)
// pair. A batch never fails wholesale on one bad pair — every pair gets its
// own result so the caller can see exactly which combinations were queued.
type EnqueueResultItem struct {
	SKU          string `json:"sku"`
	ConnectionID int64  `json:"connectionId"`
	Success      bool   `json:"success"`
	QueueID      *int64 `json:"queueId,omitempty"`
	Error        string `json:"error,omitempty"`
}

type EnqueueResult struct {
	Results []EnqueueResultItem `json:"results"`
}

type Service struct {
	db             *sql.DB
	productRepo    *mysqlInfra.ProductRepository
	connectionRepo *mysqlInfra.ChannelConnectionRepository
	queueRepo      *mysqlInfra.ChannelSyncQueueRepository
}

func NewService(
	db *sql.DB,
	productRepo *mysqlInfra.ProductRepository,
	connectionRepo *mysqlInfra.ChannelConnectionRepository,
	queueRepo *mysqlInfra.ChannelSyncQueueRepository,
) *Service {
	return &Service{
		db:             db,
		productRepo:    productRepo,
		connectionRepo: connectionRepo,
		queueRepo:      queueRepo,
	}
}

// Enqueue resolves each item's SKU to a product and writes one
// ecom_channel_sync_queue row per connectionId, so the marketplace sync
// worker(s) can later pick them up. Parent existence (product, connection)
// is validated before each insert rather than relying on the FK to reject
// it, so the caller gets a specific per-pair error instead of a raw SQL
// failure.
func (s *Service) Enqueue(ctx context.Context, input EnqueueInput) (*EnqueueResult, error) {
	if len(input.Items) == 0 {
		return nil, ErrEmptyEnqueueRequest
	}

	results := make([]EnqueueResultItem, 0)

	for _, item := range input.Items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			results = append(results, EnqueueResultItem{Error: "sku is required"})
			continue
		}

		if len(item.ConnectionIDs) == 0 {
			results = append(results, EnqueueResultItem{SKU: sku, Error: "connectionIds is required"})
			continue
		}

		product, err := s.productRepo.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				results = append(results, EnqueueResultItem{SKU: sku, Error: "product not found for sku"})
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		for _, connectionID := range item.ConnectionIDs {
			result := EnqueueResultItem{SKU: sku, ConnectionID: connectionID}

			if connectionID <= 0 {
				result.Error = "invalid connectionId"
				results = append(results, result)
				continue
			}

			if _, err := s.connectionRepo.FindByID(ctx, connectionID); err != nil {
				if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
					result.Error = "connection not found"
					results = append(results, result)
					continue
				}
				return nil, fmt.Errorf("error validating connection %d: %w", connectionID, err)
			}

			entry, err := s.queueRepo.Create(ctx, mysqlInfra.CreateChannelSyncQueueEntryInput{
				ProductID:    product.ID,
				ConnectionID: connectionID,
				UpdatedBy:    input.UpdatedBy,
			})
			if err != nil {
				result.Error = fmt.Sprintf("error enqueueing sync: %v", err)
				results = append(results, result)
				continue
			}

			result.Success = true
			result.QueueID = &entry.ID
			results = append(results, result)
		}
	}

	return &EnqueueResult{Results: results}, nil
}

package mysql

import (
	"context"
	"time"
)

// ChannelSyncQueueSummaryDTO counts a connection's queue rows of one sync_type
// by status, plus when the last one finished, so a bulk run's progress
// (done / total) can be shown while the MarketplaceWorker drains it.
type ChannelSyncQueueSummaryDTO struct {
	SyncType        string     `json:"syncType"`
	Pending         int64      `json:"pending"`
	Processing      int64      `json:"processing"`
	Done            int64      `json:"done"`
	Failed          int64      `json:"failed"`
	LastProcessedAt *time.Time `json:"lastProcessedAt,omitempty"`
}

// ChannelSyncQueueViewEntryDTO is one queue row joined with its product, for
// the connection's queue view.
type ChannelSyncQueueViewEntryDTO struct {
	ID          int64      `json:"id"`
	ProductID   int64      `json:"productId"`
	SKU         string     `json:"sku"`
	ProductName string     `json:"productName"`
	SyncType    string     `json:"syncType"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   *string    `json:"lastError,omitempty"`
	RequestedAt time.Time  `json:"requestedAt"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// SummaryByConnection returns one row per sync_type present in the connection's
// queue.
func (r *ChannelSyncQueueRepository) SummaryByConnection(ctx context.Context, connectionID int64) ([]ChannelSyncQueueSummaryDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sync_type,
		       CAST(COALESCE(SUM(status = 'pending'), 0) AS SIGNED),
		       CAST(COALESCE(SUM(status = 'processing'), 0) AS SIGNED),
		       CAST(COALESCE(SUM(status = 'done'), 0) AS SIGNED),
		       CAST(COALESCE(SUM(status = 'failed'), 0) AS SIGNED),
		       MAX(processed_at)
		FROM ecom_channel_sync_queue
		WHERE connection_id = ? AND deleted_at IS NULL
		GROUP BY sync_type
		ORDER BY sync_type
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]ChannelSyncQueueSummaryDTO, 0)
	for rows.Next() {
		var s ChannelSyncQueueSummaryDTO
		if err := rows.Scan(&s.SyncType, &s.Pending, &s.Processing, &s.Done, &s.Failed, &s.LastProcessedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// ListByConnection returns the connection's queue rows of syncType (and status,
// when non-empty), most recently updated first, with the total matching count.
func (r *ChannelSyncQueueRepository) ListByConnection(ctx context.Context, connectionID int64, syncType, status string, offset, limit int) ([]ChannelSyncQueueViewEntryDTO, int64, error) {
	where := `q.connection_id = ? AND q.sync_type = ? AND q.deleted_at IS NULL`
	args := []any{connectionID, syncType}
	if status != "" {
		where += ` AND q.status = ?`
		args = append(args, status)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ecom_channel_sync_queue q WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT q.id, q.product_id, p.sku, p.name, q.sync_type, q.status, q.attempts, q.last_error,
		       q.requested_at, q.processed_at, q.updated_at
		FROM ecom_channel_sync_queue q
		JOIN ecom_products p ON p.id = q.product_id
		WHERE `+where+`
		ORDER BY q.updated_at DESC, q.id DESC
		LIMIT ? OFFSET ?
	`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]ChannelSyncQueueViewEntryDTO, 0)
	for rows.Next() {
		var e ChannelSyncQueueViewEntryDTO
		if err := rows.Scan(&e.ID, &e.ProductID, &e.SKU, &e.ProductName, &e.SyncType, &e.Status, &e.Attempts, &e.LastError,
			&e.RequestedAt, &e.ProcessedAt, &e.UpdatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrChannelSyncQueueEntryNotFound = errors.New("channel sync queue entry not found")

const defaultChannelSyncQueueSyncType = "full"

// ChannelSyncQueueListingSyncType is the sync_type value for every row the
// listing-discovery flow produces and the marketplace consumer processes: it
// means "publish this product on this connection through channel_listings"
// (as opposed to the legacy 'full'/'price'/'stock' values). It is the only
// value written or read now that the manual /api/channel-sync-queue endpoint
// is gone.
const ChannelSyncQueueListingSyncType = "listing"

type ChannelSyncQueueDTO struct {
	ID           int64      `json:"id"`
	ProductID    int64      `json:"productId"`
	ConnectionID int64      `json:"connectionId"`
	SyncType     string     `json:"syncType"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	LastError    *string    `json:"lastError,omitempty"`
	RequestedAt  time.Time  `json:"requestedAt"`
	ProcessedAt  *time.Time `json:"processedAt,omitempty"`
	UpdatedBy    *int64     `json:"updatedBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// CreateChannelSyncQueueEntryInput enqueues one product/connection pair for
// sync. SyncType defaults to "full" (matching the column default) when
// empty. LastError is optional — set it when the entry is being queued
// because of a known, already-diagnosed precondition (e.g. no stock, no
// cover image) so that reason is visible without waiting for a worker to
// attempt and fail it.
type CreateChannelSyncQueueEntryInput struct {
	ProductID    int64
	ConnectionID int64
	SyncType     string
	LastError    string
	UpdatedBy    int64
}

type ChannelSyncQueueRepository struct {
	db Querier
}

func NewChannelSyncQueueRepository(db Querier) *ChannelSyncQueueRepository {
	return &ChannelSyncQueueRepository{db: db}
}

func (r *ChannelSyncQueueRepository) Create(ctx context.Context, input CreateChannelSyncQueueEntryInput) (*ChannelSyncQueueDTO, error) {
	syncType := input.SyncType
	if syncType == "" {
		syncType = defaultChannelSyncQueueSyncType
	}

	query := `
		INSERT INTO ecom_channel_sync_queue (product_id, connection_id, sync_type, last_error, updated_by)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.ConnectionID, syncType, nullableString(input.LastError), input.UpdatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

// FindPending returns up to limit pending 'listing' entries, oldest id first,
// for the marketplace consumer to claim and process. Legacy non-'listing'
// rows are never returned — nothing processes them anymore.
func (r *ChannelSyncQueueRepository) FindPending(ctx context.Context, limit int) ([]ChannelSyncQueueDTO, error) {
	query := `
		SELECT id, product_id, connection_id, sync_type, status, attempts, last_error,
		       requested_at, processed_at, updated_by, created_at, updated_at
		FROM ecom_channel_sync_queue
		WHERE status = 'pending' AND sync_type = '` + ChannelSyncQueueListingSyncType + `' AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]ChannelSyncQueueDTO, 0)
	for rows.Next() {
		var q ChannelSyncQueueDTO
		if err := rows.Scan(
			&q.ID, &q.ProductID, &q.ConnectionID, &q.SyncType, &q.Status, &q.Attempts, &q.LastError,
			&q.RequestedAt, &q.ProcessedAt, &q.UpdatedBy, &q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, q)
	}

	return entries, rows.Err()
}

// Claim atomically moves one entry from pending to processing, so
// concurrent workers (or worker instances) never process the same entry
// twice. It reports false, with no error, when the entry was no longer
// pending (already claimed elsewhere).
func (r *ChannelSyncQueueRepository) Claim(ctx context.Context, id int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_sync_queue
		SET status = 'processing', updated_at = NOW()
		WHERE id = ? AND status = 'pending' AND deleted_at IS NULL
	`, id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// MarkDone marks a claimed entry as successfully synced.
func (r *ChannelSyncQueueRepository) MarkDone(ctx context.Context, id, updatedBy int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_sync_queue
		SET status = 'done', processed_at = NOW(), last_error = NULL, updated_by = ?
		WHERE id = ?
	`, updatedBy, id)
	return err
}

// MarkFailed marks a claimed entry as failed and records the error, so it
// is visible without requiring the entry to be retried automatically.
func (r *ChannelSyncQueueRepository) MarkFailed(ctx context.Context, id int64, message string, updatedBy int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_sync_queue
		SET status = 'failed', attempts = attempts + 1, last_error = ?, processed_at = NOW(), updated_by = ?
		WHERE id = ?
	`, message, updatedBy, id)
	return err
}

// ReleasePending reverts a claimed entry back to pending without counting
// it as a failed attempt, for preconditions that aren't met yet (e.g. a
// product still missing its cover image or category mapping) so the entry
// is retried on a later poll instead of getting stuck as failed.
func (r *ChannelSyncQueueRepository) ReleasePending(ctx context.Context, id int64, note string, updatedBy int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_sync_queue
		SET status = 'pending', last_error = ?, updated_by = ?
		WHERE id = ?
	`, note, updatedBy, id)
	return err
}

// FindListingEntry returns the single 'listing' queue row for a
// (product, connection) pair, or ErrChannelSyncQueueEntryNotFound when there
// is none. There is at most one such row per pair — enforced by the
// uq_sync_queue_active_pair unique index (see ADR 0003) and by
// ListingDiscoveryService, which reactivates the existing row instead of
// inserting another.
func (r *ChannelSyncQueueRepository) FindListingEntry(ctx context.Context, productID, connectionID int64) (*ChannelSyncQueueDTO, error) {
	query := `
		SELECT id, product_id, connection_id, sync_type, status, attempts, last_error,
		       requested_at, processed_at, updated_by, created_at, updated_at
		FROM ecom_channel_sync_queue
		WHERE product_id = ? AND connection_id = ? AND sync_type = '` + ChannelSyncQueueListingSyncType + `' AND deleted_at IS NULL
		LIMIT 1
	`

	var q ChannelSyncQueueDTO
	if err := r.db.QueryRowContext(ctx, query, productID, connectionID).Scan(
		&q.ID, &q.ProductID, &q.ConnectionID, &q.SyncType, &q.Status, &q.Attempts, &q.LastError,
		&q.RequestedAt, &q.ProcessedAt, &q.UpdatedBy, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrChannelSyncQueueEntryNotFound
		}
		return nil, err
	}

	return &q, nil
}

// ReactivateToPending flips a failed entry back to pending so the consumer
// retries it, without touching attempts or last_error (they stay as a record
// of the prior failures). Used by ListingDiscoveryService when it re-finds a
// failed row still under the attempts cap.
func (r *ChannelSyncQueueRepository) ReactivateToPending(ctx context.Context, id, updatedBy int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_channel_sync_queue
		SET status = 'pending', processed_at = NULL, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`, updatedBy, id)
	return err
}

func (r *ChannelSyncQueueRepository) FindByID(ctx context.Context, id int64) (*ChannelSyncQueueDTO, error) {
	query := `
		SELECT id, product_id, connection_id, sync_type, status, attempts, last_error,
		       requested_at, processed_at, updated_by, created_at, updated_at
		FROM ecom_channel_sync_queue
		WHERE id = ? AND deleted_at IS NULL
	`

	var q ChannelSyncQueueDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&q.ID, &q.ProductID, &q.ConnectionID, &q.SyncType, &q.Status, &q.Attempts, &q.LastError,
		&q.RequestedAt, &q.ProcessedAt, &q.UpdatedBy, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrChannelSyncQueueEntryNotFound
		}
		return nil, err
	}

	return &q, nil
}

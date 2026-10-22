package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	importJobKeyPrefix = "meli_listing_import:job:"
	// importJobTTL is how long a job's progress stays readable after its last
	// update: long enough for the dashboard to poll it to the end and for a user
	// to come back to it, short enough that nothing piles up. Redis only holds
	// progress here; the import's real effects live in MySQL.
	importJobTTL = 2 * time.Hour
)

// ImportJobStore keeps the progress of MercadoLibre listing imports (see
// application/meli_listing_import) so every replica of the service can answer a
// status request, whichever one is running the import. Payloads are opaque.
type ImportJobStore struct {
	client *redis.Client
}

func NewImportJobStore(client *redis.Client) *ImportJobStore {
	return &ImportJobStore{client: client}
}

func (s *ImportJobStore) Save(ctx context.Context, jobID string, payload []byte) error {
	if err := s.client.Set(ctx, importJobKeyPrefix+jobID, payload, importJobTTL).Err(); err != nil {
		return fmt.Errorf("error saving import job %s to redis: %w", jobID, err)
	}
	return nil
}

func (s *ImportJobStore) Load(ctx context.Context, jobID string) ([]byte, bool, error) {
	payload, err := s.client.Get(ctx, importJobKeyPrefix+jobID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("error loading import job %s from redis: %w", jobID, err)
	}
	return payload, true, nil
}

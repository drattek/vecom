package redis

import (
	"context"
	"core-orchestrator/internal/domain"
	"encoding/json"
	redis "github.com/redis/go-redis/v9"
)

type NissanRepository struct {
	client *redis.Client
}

func NewNissanRepository(client *redis.Client) *NissanRepository {
	return &NissanRepository{client: client}
}

func (r *NissanRepository) FindByKey(ctx context.Context, key string) (*domain.Existencia, error) {
	value, err := r.client.Get(ctx, key).Result()

	if err != nil {
		return nil, err
	}

	var existencia domain.Existencia
	err = json.Unmarshal([]byte(value), &existencia)

	if err != nil {
		return nil, err
	}

	return &existencia, nil
}

func (r *NissanRepository) ScanExistencias(ctx context.Context) ([]string, error) {
	var keys []string

	iter := r.client.Scan(ctx, 0, "nissan:existencia:*", 1000).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	return keys, iter.Err()
}

// FindByKeys lee muchas claves en lotes con MGET en vez de un GET por clave: para
// decenas de miles de SKUs esto reduce los round-trips a Redis de N a N/batchSize.
func (r *NissanRepository) FindByKeys(ctx context.Context, keys []string) ([]*domain.Existencia, error) {
	const batchSize = 500

	existencias := make([]*domain.Existencia, 0, len(keys))

	for start := 0; start < len(keys); start += batchSize {
		end := start + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		values, err := r.client.MGet(ctx, keys[start:end]...).Result()
		if err != nil {
			return nil, err
		}

		for _, v := range values {
			str, ok := v.(string)
			if !ok {
				continue
			}

			var existencia domain.Existencia
			if err := json.Unmarshal([]byte(str), &existencia); err != nil {
				return nil, err
			}

			existencias = append(existencias, &existencia)
		}
	}

	return existencias, nil
}

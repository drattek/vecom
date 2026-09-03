package redis

import (
	"context"
	"core-orchestrator/internal/domain"
	"encoding/json"
	redis "github.com/redis/go-redis/v9"
)

type InventoryRepository struct {
	client *redis.Client
}

func NewInventoryRepository(client *redis.Client) *InventoryRepository {
	return &InventoryRepository{client: client}
}

func (r *InventoryRepository) FindByKey(ctx context.Context, key string) (*domain.Inventory, error) {
	value, err := r.client.Get(ctx, key).Result()

	if err != nil {
		return nil, err
	}

	var inventory domain.Inventory
	err = json.Unmarshal([]byte(value), &inventory)

	if err != nil {
		return nil, err
	}

	return &inventory, nil
}

func (r *InventoryRepository) ScanInventory(ctx context.Context) ([]string, error) {
	var keys []string

	iter := r.client.Scan(ctx, 0, "stock:*", 1000).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	return keys, iter.Err()
}

// FindByKeys lee muchas claves en lotes con MGET en vez de un GET por clave, mismo criterio
// que NissanRepository.FindByKeys.
func (r *InventoryRepository) FindByKeys(ctx context.Context, keys []string) ([]*domain.Inventory, error) {
	const batchSize = 500

	inventories := make([]*domain.Inventory, 0, len(keys))

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

			var inventory domain.Inventory
			if err := json.Unmarshal([]byte(str), &inventory); err != nil {
				return nil, err
			}

			inventories = append(inventories, &inventory)
		}
	}

	return inventories, nil
}

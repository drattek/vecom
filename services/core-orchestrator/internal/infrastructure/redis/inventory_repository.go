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

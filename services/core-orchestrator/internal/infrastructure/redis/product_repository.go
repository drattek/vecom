package redis

import (
	"context"
	"core-orchestrator/internal/domain"
	"encoding/json"
	redis "github.com/redis/go-redis/v9"
)

type ProductRepository struct {
	client *redis.Client
}

func NewProductRepository(client *redis.Client) *ProductRepository {
	return &ProductRepository{client: client}
}

func (r *ProductRepository) FindByKey(ctx context.Context, key string) (*domain.Product, error) {
	value, err := r.client.Get(ctx, key).Result()

	if err != nil {
		return nil, err
	}

	var product domain.Product
	err = json.Unmarshal([]byte(value), &product)

	if err != nil {
		return nil, err
	}

	return &product, nil
}

func (r *ProductRepository) ScanProducts(ctx context.Context) ([]string, error) {
	var keys []string

	iter := r.client.Scan(ctx, 0, "product:*", 1000).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	return keys, iter.Err()
}

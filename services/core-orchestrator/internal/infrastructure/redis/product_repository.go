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

// FindByKeys lee muchas claves en lotes con MGET en vez de un GET por clave: evita un
// round-trip a Redis por SKU cuando el catálogo ERP tiene miles de productos.
func (r *ProductRepository) FindByKeys(ctx context.Context, keys []string) ([]*domain.Product, error) {
	const batchSize = 500

	products := make([]*domain.Product, 0, len(keys))

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

			var product domain.Product
			if err := json.Unmarshal([]byte(str), &product); err != nil {
				return nil, err
			}

			products = append(products, &product)
		}
	}

	return products, nil
}

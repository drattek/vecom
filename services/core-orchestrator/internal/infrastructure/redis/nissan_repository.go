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

// DeleteKeys borra en lotes las claves ya consumidas. El sync de existencias las
// vuelve a escribir en cada corrida solo para los SKU con stock > 0, así que si
// no se borran las que dejaron de venir quedan como "fantasmas" con el último
// valor positivo y ScanExistencias las seguiría re-sincronizando para siempre.
// Un error acá no debe abortar el sync: la baja real del stock ya la resuelve la
// reconciliación por ausencia contra MySQL (ver sync_stock_reconcile.go).
func (r *NissanRepository) DeleteKeys(ctx context.Context, keys []string) error {
	const batchSize = 500

	for start := 0; start < len(keys); start += batchSize {
		end := min(start+batchSize, len(keys))

		if err := r.client.Del(ctx, keys[start:end]...).Err(); err != nil {
			return err
		}
	}

	return nil
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

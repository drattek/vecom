package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const oauthStateKeyPrefix = "meli_oauth:state:"

// OAuthStateStore guarda, mientras el usuario está en MercadoLibre, los datos
// de una autorización en curso (ver sync.MercadoLibreOAuthService). Son
// temporales: perderlos solo obliga a iniciar la vinculación otra vez.
type OAuthStateStore struct {
	client *redis.Client
}

func NewOAuthStateStore(client *redis.Client) *OAuthStateStore {
	return &OAuthStateStore{client: client}
}

func (s *OAuthStateStore) Save(ctx context.Context, state string, payload []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, oauthStateKeyPrefix+state, payload, ttl).Err(); err != nil {
		return fmt.Errorf("error saving oauth state to redis: %w", err)
	}
	return nil
}

// Take lee y elimina el state en una sola operación: cada uno sirve una vez.
// Devuelve false si no existe, caducó o ya se usó.
func (s *OAuthStateStore) Take(ctx context.Context, state string) ([]byte, bool, error) {
	payload, err := s.client.GetDel(ctx, oauthStateKeyPrefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("error reading oauth state from redis: %w", err)
	}
	return payload, true, nil
}

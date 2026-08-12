package redis

import (
	"fmt"

	"core-orchestrator/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewClient() *redis.Client {
	cfg := config.Load()

	addr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)

	return redis.NewClient(
		&redis.Options{
			Addr: addr,
			DB:   0,
		},
	)
}

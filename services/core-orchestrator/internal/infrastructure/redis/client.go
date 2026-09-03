package redis

import (
	"fmt"

	"core-orchestrator/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewClient(cfg config.Config) *redis.Client {
	addr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)

	return redis.NewClient(
		&redis.Options{
			Addr: addr,
			DB:   0,
		},
	)
}

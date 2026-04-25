package cache

import (
	"context"
	"go-lobby/config"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(config *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})
}

func PingRedis(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

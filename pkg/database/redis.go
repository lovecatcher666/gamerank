package database

import (
	"context"
	"fmt"
	"time"

	"game-leaderboard/pkg/logger"

	"github.com/go-redis/redis/v8"
)

func NewRedisConnection(addr, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 100,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.NewLogger("database").Info("Redis connection established")
	return client, nil
}

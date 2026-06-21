package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lkmavi/team-tasks/internal/config"
)

const (
	maxRetries    = 10
	retryInterval = 3 * time.Second
)

// New creates a *redis.Client with the configured pool and verifies connectivity.
// It retries up to maxRetries times to tolerate the Redis container starting after
// the app container.
func New(cfg *config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Ping(ctx).Result()
		cancel()

		if err == nil {
			return client, nil
		}

		lastErr = err
		slog.Warn("redis: not ready, retrying",
			"attempt", attempt,
			"of", maxRetries,
			"backoff", retryInterval,
			"error", err.Error(),
		)
		time.Sleep(retryInterval)
	}

	_ = client.Close()
	return nil, fmt.Errorf("redis: unavailable after %d attempts: %w", maxRetries, lastErr)
}

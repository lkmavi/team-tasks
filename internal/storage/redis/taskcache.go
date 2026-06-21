package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// ErrCacheMiss is returned by Get when the key does not exist in the cache.
var ErrCacheMiss = errors.New("cache miss")

// TaskCache is a Redis-backed cache for task list query results.
type TaskCache struct {
	client *redis.Client
}

// NewTaskCache creates a TaskCache using the provided Redis client.
func NewTaskCache(client *redis.Client) *TaskCache {
	return &TaskCache{client: client}
}

func (c *TaskCache) Get(ctx context.Context, key string) ([]byte, error) {
	const op = "redis.TaskCache.Get"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("key", key))

	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		log.Info("cache miss")
		return nil, ErrCacheMiss
	}
	if err != nil {
		log.Error("failed to get cache entry", slogx.Err(err))
		return nil, fmt.Errorf("redis: get %q: %w", key, err)
	}

	log.Info("cache hit")
	return data, nil
}

func (c *TaskCache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	const op = "redis.TaskCache.Set"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("key", key))
	log.Info("setting cache entry")

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		log.Error("failed to set cache entry", slogx.Err(err))
		return fmt.Errorf("redis: set %q: %w", key, err)
	}

	log.Info("cache entry set")
	return nil
}

// Invalidate deletes all keys matching the given pattern.
// Pattern may end with '*' for prefix-match via SCAN; otherwise an exact key is deleted.
func (c *TaskCache) Invalidate(ctx context.Context, pattern string) error {
	const op = "redis.TaskCache.Invalidate"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("pattern", pattern))
	log.Info("invalidating cache")

	if !strings.HasSuffix(pattern, "*") {
		if err := c.client.Del(ctx, pattern).Err(); err != nil {
			log.Error("failed to delete cache key", slogx.Err(err))
			return fmt.Errorf("redis: del %q: %w", pattern, err)
		}
		log.Info("cache key deleted")
		return nil
	}

	var cursor uint64
	var deleted int
	for {
		var keys []string
		var err error
		keys, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Error("failed to scan cache keys", slogx.Err(err))
			return fmt.Errorf("redis: scan %q: %w", pattern, err)
		}
		if len(keys) > 0 {
			if err = c.client.Del(ctx, keys...).Err(); err != nil {
				log.Error("failed to delete cache keys", slogx.Err(err))
				return fmt.Errorf("redis: del keys: %w", err)
			}
			deleted += len(keys)
		}
		if cursor == 0 {
			break
		}
	}

	log.Info("cache invalidated", slog.Int("keys_deleted", deleted))
	return nil
}

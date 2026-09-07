package tagcache

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	corecache "github.com/hangry-coder/bffx/pkg/cache"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = corecache.RedisKeyPrefix
	tagPrefix = corecache.RedisTagPrefix
)

// RedisTagCache implements the Tagger interface using Redis.
// It uses Redis Sets to track which cache keys are associated with which tags.
type RedisTagCache struct {
	client *redis.Client
}

// NewRedisTagCache creates a new Redis-backed tag cache.
func NewRedisTagCache(client *redis.Client) *RedisTagCache {
	return &RedisTagCache{
		client: client,
	}
}

// Get retrieves a value from Redis. If Redis is down, it fails open (returns MISS).
func (c *RedisTagCache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, nil
	}

	val, err := c.client.Get(ctx, keyPrefix+key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		logger.Error("tagcache: redis get error (failing open) key=%s err=%v", key, err)
		return nil, nil // Fail open
	}

	return val, nil
}

// Set stores a value and registers it under multiple tags using a Redis pipeline.
func (c *RedisTagCache) Set(ctx context.Context, key string, body []byte, tags []string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}

	fullKey := keyPrefix + key
	pipe := c.client.Pipeline()

	// 1. Store the actual data
	pipe.Set(ctx, fullKey, body, ttl)

	// 2. Add this key to each tag's set
	for _, tag := range tags {
		fullTag := tagPrefix + tag
		pipe.SAdd(ctx, fullTag, fullKey)
		// Set tag set TTL slightly longer than entry TTL to avoid orphan sets growing forever,
		// though SMEMBERS + UNLINK is the primary cleanup.
		pipe.Expire(ctx, fullTag, ttl+time.Hour)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("tagcache: redis set pipeline error key=%s tags=%v err=%v", key, tags, err)
		return err
	}

	return nil
}

// InvalidateTags unlinks all keys associated with the given tags.
func (c *RedisTagCache) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	if c.client == nil {
		return 0, nil
	}

	totalDeleted := 0
	for _, tag := range tags {
		fullTag := tagPrefix + tag
		
		// 1. Get all keys for this tag
		keys, err := c.client.SMembers(ctx, fullTag).Result()
		if err != nil {
			logger.Error("tagcache: redis smembers error during invalidation tag=%s err=%v", tag, err)
			return totalDeleted, err
		}

		if len(keys) == 0 {
			continue
		}

		// 2. Unlink the keys in chunks to avoid blocking Redis
		const chunkSize = 500
		for i := 0; i < len(keys); i += chunkSize {
			end := i + chunkSize
			if end > len(keys) {
				end = len(keys)
			}
			
			// UNLINK is non-blocking (Redis 4.0+)
			numDeleted, err := c.client.Unlink(ctx, keys[i:end]...).Result()
			if err != nil {
				logger.Error("tagcache: redis unlink error tag=%s err=%v", tag, err)
				// We continue to try invalidating the rest or at least delete the tag set
			}
			totalDeleted += int(numDeleted)
		}

		// 3. Remove the tag set itself
		c.client.Del(ctx, fullTag)
	}

	return totalDeleted, nil
}

func (c *RedisTagCache) Name() string {
	return "redis"
}

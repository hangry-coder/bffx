package cache

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisKeyPrefix = "bffx:cache:k:"
	redisTagPrefix = "bffx:cache:t:"
)

// RedisCache implements the Provider interface using Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis-backed cache.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, nil
	}

	val, err := c.client.Get(ctx, redisKeyPrefix+key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		logger.Error("cache: redis get error key=%s err=%v", key, err)
		return nil, nil // Fail open
	}

	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.SetWithTags(ctx, key, value, nil, ttl)
}

func (c *RedisCache) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}

	fullKey := redisKeyPrefix + key
	pipe := c.client.Pipeline()

	// 1. Store the actual data
	pipe.Set(ctx, fullKey, value, ttl)

	// 2. Add this key to each tag's set
	for _, tag := range tags {
		fullTag := redisTagPrefix + tag
		pipe.SAdd(ctx, fullTag, fullKey)
		// Set tag set TTL slightly longer than entry TTL
		pipe.Expire(ctx, fullTag, ttl+time.Hour)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("cache: redis set pipeline error key=%s tags=%v err=%v", key, tags, err)
		return err
	}

	return nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(ctx, redisKeyPrefix+key).Err()
}

func (c *RedisCache) Flush(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	// Warning: FlushDB is destructive for the whole DB
	return c.client.FlushDB(ctx).Err()
}

func (c *RedisCache) Tags(tags ...string) Tagger {
	return &redisTagger{
		cache: c,
		tags:  tags,
	}
}

func (c *RedisCache) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	if c.client == nil {
		return 0, nil
	}

	totalDeleted := 0
	for _, tag := range tags {
		fullTag := redisTagPrefix + tag
		
		keys, err := c.client.SMembers(ctx, fullTag).Result()
		if err != nil {
			logger.Error("cache: redis smembers error tag=%s err=%v", tag, err)
			continue
		}

		if len(keys) == 0 {
			continue
		}

		// UNLINK is non-blocking
		numDeleted, err := c.client.Unlink(ctx, keys...).Result()
		if err != nil {
			logger.Error("cache: redis unlink error tag=%s err=%v", tag, err)
		}
		totalDeleted += int(numDeleted)

		// Remove the tag set itself
		c.client.Del(ctx, fullTag)
	}

	return totalDeleted, nil
}

func (c *RedisCache) Type() string {
	return "redis"
}

func (c *RedisCache) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	if c.client == nil {
		return 0, nil
	}

	// WARNING: KEYS can be slow on large datasets. In production, SCAN is preferred.
	// But for our battery-level invalidation, we prefix keys.
	fullPattern := redisKeyPrefix + pattern
	keys, err := c.client.Keys(ctx, fullPattern).Result()
	if err != nil {
		return 0, err
	}

	if len(keys) == 0 {
		return 0, nil
	}

	numDeleted, err := c.client.Unlink(ctx, keys...).Result()
	return int(numDeleted), err
}

func (c *RedisCache) Ping(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	return c.client.Ping(ctx).Err()
}

type redisTagger struct {
	cache *RedisCache
	tags  []string
}

func (t *redisTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return t.cache.Get(ctx, key)
}

func (t *redisTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return t.cache.SetWithTags(ctx, key, value, t.tags, ttl)
}

package cache

import (
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisProvider implements the cache.Provider interface using Redis.
type RedisProvider struct {
	client *redis.Client
}

// NewRedisProvider creates a new Redis-backed cache battery.
func NewRedisProvider(client *redis.Client) *RedisProvider {
	return &RedisProvider{
		client: client,
	}
}

func (p *RedisProvider) Get(ctx context.Context, key string) ([]byte, error) {
	if p.client == nil {
		return nil, nil
	}

	val, err := p.client.Get(ctx, cache.RedisDataKey(key)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		logger.Error("cache: redis get error key=%s err=%v", key, err)
		return nil, nil // Fail open
	}

	return val, nil
}

func (p *RedisProvider) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return p.SetWithTags(ctx, key, value, nil, ttl)
}

func (p *RedisProvider) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	if p.client == nil {
		return nil
	}

	fullKey := cache.RedisDataKey(key)
	pipe := p.client.Pipeline()

	// 1. Store the actual data
	pipe.Set(ctx, fullKey, value, ttl)

	// 2. Add this key to each tag's set
	for _, tag := range tags {
		fullTag := cache.RedisTagKey(tag)
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

func (p *RedisProvider) Delete(ctx context.Context, key string) error {
	if p.client == nil {
		return nil
	}
	return p.client.Del(ctx, cache.RedisDataKey(key)).Err()
}

func (p *RedisProvider) Flush(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	return p.client.FlushDB(ctx).Err()
}

func (p *RedisProvider) Tags(tags ...string) cache.Tagger {
	return &redisTagger{
		provider: p,
		tags:     tags,
	}
}

func (p *RedisProvider) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	if p.client == nil {
		return 0, nil
	}

	totalDeleted := 0
	for _, tag := range tags {
		fullTag := cache.RedisTagKey(tag)
		
		keys, err := p.client.SMembers(ctx, fullTag).Result()
		if err != nil {
			logger.Error("cache: redis smembers error tag=%s err=%v", tag, err)
			continue
		}

		if len(keys) == 0 {
			continue
		}

		numDeleted, err := p.client.Unlink(ctx, keys...).Result()
		if err != nil {
			logger.Error("cache: redis unlink error tag=%s err=%v", tag, err)
		}
		totalDeleted += int(numDeleted)

		p.client.Del(ctx, fullTag)
	}

	return totalDeleted, nil
}

func (p *RedisProvider) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	if p.client == nil {
		return 0, nil
	}

	// Keys are stored as RedisDataKey(logicalKey) (see Get/Set).
	fullPattern := cache.RedisDataKey(pattern)
	totalDeleted := 0
	var cursor uint64
	for {
		keys, nextCursor, err := p.client.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			return totalDeleted, err
		}
		if len(keys) > 0 {
			deleted, _ := p.client.Unlink(ctx, keys...).Result()
			totalDeleted += int(deleted)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return totalDeleted, nil
}

func (p *RedisProvider) Type() string {
	return "redis"
}

func (p *RedisProvider) Ping(ctx context.Context) error {
	if p.client == nil {
		return context.DeadlineExceeded // Or some other error indicating nil client
	}
	return p.client.Ping(ctx).Err()
}

type redisTagger struct {
	provider *RedisProvider
	tags     []string
}

func (t *redisTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return t.provider.Get(ctx, key)
}

func (t *redisTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return t.provider.SetWithTags(ctx, key, value, t.tags, ttl)
}

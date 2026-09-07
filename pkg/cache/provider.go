package cache

import (
	"context"
	"time"
)

const (
	// Redis data/index namespaces used by cache providers.
	RedisKeyPrefix = "bffx:cache:k:"
	RedisTagPrefix = "bffx:cache:t:"

	// Logical HTTP cache key namespaces (before provider prefixes).
	ActionResponseKeyPrefix = "bffx:action-cache:"
	TaggedResponseKeyPrefix = "bffx:tagcache:"

	// Secondary in-provider tag index namespace.
	TagIndexPrefix = "cache_index:"
)

// RedisDataKey returns the provider-level storage key for a logical cache key.
func RedisDataKey(logicalKey string) string {
	return RedisKeyPrefix + logicalKey
}

// RedisTagKey returns the provider-level storage key for a logical tag.
func RedisTagKey(tag string) string {
	return RedisTagPrefix + tag
}

// TagIndexKey returns the secondary index key used to track keys per tag.
func TagIndexKey(tag string) string {
	return TagIndexPrefix + tag
}

// Tagger defines the interface for tagged cache operations.
type Tagger interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Provider defines the interface for a pluggable cache battery.
type Provider interface {
	// Basic Operations
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Flush(ctx context.Context) error

	// Tagged Operations
	// Tags returns a Tagger that will associate all Set operations with the given tags.
	Tags(tags ...string) Tagger
	
	// SetWithTags stores a value and associates it with the provided tags.
	SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error

	// InvalidateTags removes all entries associated with any of the given tags.
	InvalidateTags(ctx context.Context, tags []string) (int, error)

	// InvalidatePattern removes all entries matching a glob pattern.
	InvalidatePattern(ctx context.Context, pattern string) (int, error)

	// Type returns the battery type (e.g., "memory", "redis", "upstash").
	Type() string

	// Ping checks connectivity to the cache backend.
	Ping(ctx context.Context) error
}

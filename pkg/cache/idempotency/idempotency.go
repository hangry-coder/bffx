package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
)

const keyPrefix = "idemp:"

// Response represents a cached HTTP response.
type Response struct {
	StatusCode int                 `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

// Store defines the interface for storing and retrieving idempotency responses.
type Store interface {
	Get(ctx context.Context, key string) (*Response, error)
	Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error
}

// DurableAware is implemented by stores that can report whether writes are
// expected to survive process restarts (e.g. Redis-backed stores).
type DurableAware interface {
	Durable() bool
}

// IsDurable reports whether the provided store is durable.
func IsDurable(store Store) bool {
	if store == nil {
		return false
	}
	if d, ok := store.(DurableAware); ok {
		return d.Durable()
	}
	return false
}

// CacheStore implements Store using a cache.Provider battery.
type CacheStore struct {
	provider cache.Provider
}

func NewCacheStore(provider cache.Provider) *CacheStore {
	return &CacheStore{provider: provider}
}

func (s *CacheStore) Get(ctx context.Context, key string) (*Response, error) {
	if s.provider == nil {
		return nil, nil
	}

	val, err := s.provider.Get(ctx, keyPrefix+key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, nil
	}

	var resp Response
	if err := json.Unmarshal(val, &resp); err != nil {
		logger.Error("idempotency: unmarshal error key=%s err=%v", key, err)
		return nil, err
	}
	return &resp, nil
}

func (s *CacheStore) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	if s.provider == nil {
		return nil
	}

	val, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return s.provider.Set(ctx, keyPrefix+key, val, ttl)
}

// Durable reports whether the cache backend is expected to survive process restarts.
func (s *CacheStore) Durable() bool {
	if s.provider == nil {
		return false
	}
	return s.provider.Type() != "memory"
}

package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	value      []byte
	expiration int64
}

// LocalCache implements Provider in-memory.
type LocalCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	tags    map[string]map[string]struct{} // tag -> set of keys
}

func NewLocalCache() *LocalCache {
	c := &LocalCache{
		entries: make(map[string]cacheEntry),
		tags:    make(map[string]map[string]struct{}),
	}
	go c.cleanupLoop()
	return c
}

func (c *LocalCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || (entry.expiration > 0 && time.Now().UnixNano() > entry.expiration) {
		return nil, nil
	}
	return entry.value, nil
}

func (c *LocalCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.SetWithTags(ctx, key, value, nil, ttl)
}

func (c *LocalCache) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	c.entries[key] = cacheEntry{
		value:      value,
		expiration: exp,
	}

	for _, tag := range tags {
		if _, ok := c.tags[tag]; !ok {
			c.tags[tag] = make(map[string]struct{})
		}
		c.tags[tag][key] = struct{}{}
	}

	return nil
}

func (c *LocalCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

func (c *LocalCache) Flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
	c.tags = make(map[string]map[string]struct{})
	return nil
}

func (c *LocalCache) Tags(tags ...string) Tagger {
	return &localTagger{
		cache: c,
		tags:  tags,
	}
}

func (c *LocalCache) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := 0
	for _, tag := range tags {
		if keys, ok := c.tags[tag]; ok {
			for key := range keys {
				if _, exists := c.entries[key]; exists {
					delete(c.entries, key)
					deleted++
				}
			}
			delete(c.tags, tag)
		}
	}
	return deleted, nil
}

func (c *LocalCache) Type() string {
	return "memory"
}

func (c *LocalCache) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := 0
	// Simple pattern matching (contains for now, or could use path.Match)
	for key := range c.entries {
		if strings.Contains(key, pattern) {
			delete(c.entries, key)
			deleted++
		}
	}
	return deleted, nil
}

func (c *LocalCache) Ping(ctx context.Context) error {
	return nil
}

func (c *LocalCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now().UnixNano()
		for k, v := range c.entries {
			if v.expiration > 0 && now > v.expiration {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

type localTagger struct {
	cache *LocalCache
	tags  []string
}

func (t *localTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return t.cache.Get(ctx, key)
}

func (t *localTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return t.cache.SetWithTags(ctx, key, value, t.tags, ttl)
}

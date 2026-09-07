package cache

import (
	"github.com/hangry-coder/bffx/pkg/cache"
	"context"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	value      []byte
	expiration int64
}

// MemoryProvider implements cache.Provider in-memory.
type MemoryProvider struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	tags    map[string]map[string]struct{} // tag -> set of keys
}

func NewMemoryProvider() *MemoryProvider {
	p := &MemoryProvider{
		entries: make(map[string]cacheEntry),
		tags:    make(map[string]map[string]struct{}),
	}
	go p.cleanupLoop()
	return p
}

func (p *MemoryProvider) Get(ctx context.Context, key string) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.entries[key]
	if !ok || (entry.expiration > 0 && time.Now().UnixNano() > entry.expiration) {
		return nil, nil
	}
	return entry.value, nil
}

func (p *MemoryProvider) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return p.SetWithTags(ctx, key, value, nil, ttl)
}

func (p *MemoryProvider) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	p.entries[key] = cacheEntry{
		value:      value,
		expiration: exp,
	}

	for _, tag := range tags {
		if _, ok := p.tags[tag]; !ok {
			p.tags[tag] = make(map[string]struct{})
		}
		p.tags[tag][key] = struct{}{}
	}

	return nil
}

func (p *MemoryProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.entries, key)
	return nil
}

func (p *MemoryProvider) Flush(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = make(map[string]cacheEntry)
	p.tags = make(map[string]map[string]struct{})
	return nil
}

func (p *MemoryProvider) Tags(tags ...string) cache.Tagger {
	return &memoryTagger{
		provider: p,
		tags:     tags,
	}
}

func (p *MemoryProvider) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	deleted := 0
	for _, tag := range tags {
		if keys, ok := p.tags[tag]; ok {
			for key := range keys {
				if _, exists := p.entries[key]; exists {
					delete(p.entries, key)
					deleted++
				}
			}
			delete(p.tags, tag)
		}
	}
	return deleted, nil
}

func (p *MemoryProvider) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	deleted := 0
	// Use simple prefix match for now as that's the primary use case
	prefix := strings.TrimSuffix(pattern, "*")
	for k := range p.entries {
		if strings.HasPrefix(k, prefix) {
			delete(p.entries, k)
			deleted++
		}
	}
	return deleted, nil
}

func (p *MemoryProvider) Type() string {
	return "memory"
}

func (p *MemoryProvider) Ping(ctx context.Context) error {
	return nil
}

func (p *MemoryProvider) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		p.mu.Lock()
		now := time.Now().UnixNano()
		for k, v := range p.entries {
			if v.expiration > 0 && now > v.expiration {
				delete(p.entries, k)
			}
		}
		p.mu.Unlock()
	}
}

type memoryTagger struct {
	provider *MemoryProvider
	tags     []string
}

func (t *memoryTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return t.provider.Get(ctx, key)
}

func (t *memoryTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return t.provider.SetWithTags(ctx, key, value, t.tags, ttl)
}

package idempotency

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	resp      *Response
	expiresAt time.Time
}

// MemoryStore implements Store using in-memory map.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]memoryEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]memoryEntry),
	}
}

func (s *MemoryStore) Get(ctx context.Context, key string) (*Response, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	return entry.resp, nil
}

func (s *MemoryStore) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = memoryEntry{
		resp:      resp,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Durable reports whether this store survives process restarts.
func (s *MemoryStore) Durable() bool { return false }

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hangry-coder/bffx/pkg/cache"
)

// Message represents a single turn in a chat interaction.
type Message struct {
	Role    string `json:"role"` // "user", "model", "system"
	Content string `json:"content"`
}

// MemoryManager manages chat sessions using the framework's cache provider.
type MemoryManager struct {
	cache      cache.Provider
	maxHistory int
	ttl        time.Duration
}

// NewMemoryManager creates a MemoryManager instance.
func NewMemoryManager(cache cache.Provider, maxHistory int, ttl time.Duration) *MemoryManager {
	if maxHistory <= 0 {
		maxHistory = 10
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &MemoryManager{
		cache:      cache,
		maxHistory: maxHistory,
		ttl:        ttl,
	}
}

// GetHistory retrieves the chat history for a session.
func (m *MemoryManager) GetHistory(ctx context.Context, sessionID string) ([]Message, error) {
	if sessionID == "" {
		return nil, nil
	}
	key := fmt.Sprintf("bffx:chat:session:%s", sessionID)
	data, err := m.cache.Get(ctx, key)
	if err != nil || len(data) == 0 {
		return nil, nil // Safe fallback for cache miss or empty cache value
	}

	var history []Message
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// Append appends a message to the session's sliding history window.
func (m *MemoryManager) Append(ctx context.Context, sessionID string, msg Message) ([]Message, error) {
	if sessionID == "" {
		return nil, nil
	}

	history, err := m.GetHistory(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	history = append(history, msg)

	// Apply sliding window constraint
	if len(history) > m.maxHistory {
		history = history[len(history)-m.maxHistory:]
	}

	key := fmt.Sprintf("bffx:chat:session:%s", sessionID)
	data, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}

	if err := m.cache.Set(ctx, key, data, m.ttl); err != nil {
		return nil, err
	}

	return history, nil
}

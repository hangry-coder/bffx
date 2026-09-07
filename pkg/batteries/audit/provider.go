package audit

import (
	"context"
	"time"
)

// Level defines the severity of the audit event.
type Level string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

// Entry represents a single audit log record.
type Entry struct {
	ID         string         `json:"id"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     string         `json:"user_id,omitempty"`
	Action     string         `json:"action"` // e.g., "create", "delete", "login"
	Resource   string         `json:"resource,omitempty"`
	ResourceID string         `json:"resource_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Level      Level          `json:"level"`
	IP         string         `json:"ip,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
}

// Provider defines the interface for the Audit Log battery.
type Provider interface {
	// Log records an audit entry.
	Log(ctx context.Context, entry Entry) error
	
	// Query retrieves audit logs based on filters.
	Query(ctx context.Context, filter map[string]any, limit, offset int) ([]Entry, error)

	// Type returns the provider type.
	Type() string
}

// MemoryProvider is a simple in-memory implementation for dev/testing.
type MemoryProvider struct {
	entries []Entry
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{entries: make([]Entry, 0)}
}

func (p *MemoryProvider) Log(ctx context.Context, entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	p.entries = append(p.entries, entry)
	return nil
}

func (p *MemoryProvider) Query(ctx context.Context, filter map[string]any, limit, offset int) ([]Entry, error) {
	// Simple slice operations for memory provider
	if offset > len(p.entries) {
		return []Entry{}, nil
	}
	end := offset + limit
	if end > len(p.entries) {
		end = len(p.entries)
	}
	return p.entries[offset:end], nil
}

func (p *MemoryProvider) Type() string {
	return "memory"
}

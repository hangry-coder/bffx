package tagcache

import (
	"context"
	corecache "github.com/hangry-coder/bffx/pkg/cache"
	"time"
)

// Tagger defines the interface for a tagged response cache.
// It allows setting cache entries with multiple tags and invalidating 
// all entries associated with specific tags.
type Tagger interface {
	// Get retrieves a cached body by key. 
	// Returns (nil, nil) if the key is not found (MISS).
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a body with a given key and associates it with the provided tags.
	// The entry will expire after the specified TTL.
	Set(ctx context.Context, key string, body []byte, tags []string, ttl time.Duration) error

	// InvalidateTags removes all cache entries associated with any of the provided tags.
	// Returns the number of keys successfully unlinked.
	InvalidateTags(ctx context.Context, tags []string) (int, error)

	// Name returns the provider name (e.g., "redis", "memory").
	Name() string
}

// ScreenTag returns the canonical tag for a screen response.
func ScreenTag(screen string) string {
	return corecache.ScreenTag(screen)
}

// SectionTag returns the canonical tag for a section response.
func SectionTag(screen, section string) string {
	return corecache.SectionTag(screen, section)
}

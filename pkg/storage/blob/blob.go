package blob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotConfigured indicates required environment variables for an implementation are missing.
var ErrNotConfigured = errors.New("blob: storage not configured")

// PresignResult is returned by Provider implementations for clients that PUT/GET objects.
type PresignResult struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// Provider defines the interface for a pluggable blob battery.
type Provider interface {
	// PresignPut issues a time-limited URL for direct client uploads.
	PresignPut(ctx context.Context, baseURL, objectKey string, ttl time.Duration) (PresignResult, error)

	// PresignGet issues a time-limited URL for direct client downloads.
	PresignGet(ctx context.Context, baseURL, objectKey string, ttl time.Duration) (PresignResult, error)

	// PublicURL returns a permanent or semi-permanent public URL for an object if supported.
	PublicURL(baseURL, objectKey string) string

	// Delete removes an object from storage.
	Delete(ctx context.Context, objectKey string) error

	// Type returns the battery type (e.g., "local", "s3", "r2").
	Type() string

	// Ping checks connectivity to the storage backend.
	Ping(ctx context.Context) error
}

// ValidateObjectKey rejects empty, oversized, or traversal-like keys.
func ValidateObjectKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 512 {
		return fmt.Errorf("blob: invalid object key length")
	}
	if strings.Contains(key, "..") {
		return fmt.Errorf("blob: object key must not contain ..")
	}
	return nil
}

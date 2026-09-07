package analytics

import "context"

// Provider defines the interface for the analytics battery.
type Provider interface {
	// Type returns the name of the provider (e.g., "noop", "posthog").
	Type() string
	// Track records an analytics event.
	Track(ctx context.Context, event string, properties map[string]any)
	// Close flushes any buffered events and stops the provider.
	Close(ctx context.Context) error
}

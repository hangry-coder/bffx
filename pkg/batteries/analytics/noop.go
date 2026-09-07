package analytics

import "context"

// NoopProvider is a fallback analytics provider that does nothing.
type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) Type() string {
	return "noop"
}

func (p *NoopProvider) Track(ctx context.Context, event string, properties map[string]any) {
	// Do nothing
}

func (p *NoopProvider) Close(ctx context.Context) error {
	return nil
}

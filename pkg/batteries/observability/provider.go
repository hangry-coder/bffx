package observability

import (
	"log/slog"
)

// Provider is the log-shipping battery adapter (slog/axiom/sentry).
// Core telemetry semantics (headers, metric keys, audit shapes) live in
// github.com/hangry-coder/bffx/pkg/observability.
type Provider interface {
	// Type returns the name of the provider (e.g., "slog", "axiom").
	Type() string
	// Logger returns the slog.Logger instance configured for this provider.
	Logger() *slog.Logger
	// Sync flushes any buffered logs.
	Sync() error
}

// LogProvider is the canonical name for the logging battery adapter interface.
type LogProvider = Provider

package observability

import (
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	coreobs "github.com/hangry-coder/bffx/pkg/observability"
)

type SentryProvider struct {
	logger *slog.Logger
}

func NewSentryProvider(dsn string) *SentryProvider {
	opts := sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      os.Getenv("BFFX_ENV"),
		Release:          os.Getenv("BFFX_RELEASE"),
		AttachStacktrace: true,
	}
	
	if err := sentry.Init(opts); err != nil {
		// If sentry fails to init, we just log it and continue with slog
		slog.Error("failed to initialize Sentry", "error", err)
	}

	return &SentryProvider{
		logger: slog.Default(), // We can enhance this with a Sentry-specific handler if needed
	}
}

func (p *SentryProvider) Type() string {
	return coreobs.LogProviderSentry
}

func (p *SentryProvider) Logger() *slog.Logger {
	return p.logger
}

func (p *SentryProvider) Sync() error {
	sentry.Flush(2 * time.Second)
	return nil
}

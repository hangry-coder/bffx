package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	coreobs "github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/logger"
)

// SlogProvider is the default observability provider using the standard log/slog package.
type SlogProvider struct {
	logger *slog.Logger
}

// NewSlogProvider creates a new SlogProvider.
// It configures a JSON handler writing to stdout.
func NewSlogProvider() *SlogProvider {
	level := slog.LevelInfo
	if strings.ToLower(os.Getenv("BFFX_LOG_LEVEL")) == "debug" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("BFFX_LOG_FORMAT")) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	// Wrap with TraceID support if extractor is available
	handler = &ContextHandler{Handler: handler}

	return &SlogProvider{
		logger: slog.New(handler),
	}
}

func (p *SlogProvider) Type() string {
	return coreobs.LogProviderSlog
}

func (p *SlogProvider) Logger() *slog.Logger {
	return p.logger
}

func (p *SlogProvider) Sync() error {
	return nil
}

// ContextHandler wraps an existing slog.Handler to extract TraceID from context.
type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if logger.TraceIDExtractor != nil && ctx != nil {
		if tid := logger.TraceIDExtractor(ctx); tid != "" {
			r.AddAttrs(slog.String(coreobs.FieldTraceID, tid))
		}
	}
	return h.Handler.Handle(ctx, r)
}

package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	coreobs "github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/logger"
)

const (
	axiomDefaultURL    = "https://api.axiom.co/v1/datasets/%s/ingest"
	axiomFlushInterval = 5 * time.Second
	axiomBufferSize    = 50
)

// AxiomProvider implements the observability.Provider interface for Axiom.
type AxiomProvider struct {
	logger  *slog.Logger
	handler *AxiomHandler
}

// AxiomHandler is an slog.Handler that sends logs to Axiom.
type AxiomHandler struct {
	dataset string
	token   string
	url     string
	client  *http.Client
	next    slog.Handler // Local fallback (slog)

	mu     sync.Mutex
	buffer []map[string]any
	stop   chan struct{}
}

func NewAxiomProvider(dataset, token string) *AxiomProvider {
	localHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	
	h := &AxiomHandler{
		dataset: dataset,
		token:   token,
		url:     fmt.Sprintf(axiomDefaultURL, dataset),
		client:  &http.Client{Timeout: 10 * time.Second},
		next:    localHandler,
		buffer:  make([]map[string]any, 0, axiomBufferSize),
		stop:    make(chan struct{}),
	}

	go h.run()

	return &AxiomProvider{
		logger:  slog.New(h),
		handler: h,
	}
}

func (p *AxiomProvider) Type() string {
	return coreobs.LogProviderAxiom
}

func (p *AxiomProvider) Logger() *slog.Logger {
	return p.logger
}

func (p *AxiomProvider) Sync() error {
	return p.handler.Flush()
}

func (h *AxiomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *AxiomHandler) Handle(ctx context.Context, r slog.Record) error {
	// Always log to local stdout first
	if err := h.next.Handle(ctx, r); err != nil {
		return err
	}

	// Prepare map for Axiom
	m := make(map[string]any)
	m["_time"] = r.Time.Format(time.RFC3339Nano)
	m["level"] = r.Level.String()
	m["message"] = r.Message
	
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value.Any()
		return true
	})

	// Add trace_id from context if available
	if logger.TraceIDExtractor != nil && ctx != nil {
		if tid := logger.TraceIDExtractor(ctx); tid != "" {
			m[coreobs.FieldTraceID] = tid
		}
	}

	h.mu.Lock()
	h.buffer = append(h.buffer, m)
	shouldFlush := len(h.buffer) >= axiomBufferSize
	h.mu.Unlock()

	if shouldFlush {
		return h.Flush()
	}

	return nil
}

func (h *AxiomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &AxiomHandler{
		dataset: h.dataset,
		token:   h.token,
		url:     h.url,
		client:  h.client,
		next:    h.next.WithAttrs(attrs),
		buffer:  h.buffer,
		stop:    h.stop,
	}
}

func (h *AxiomHandler) WithGroup(name string) slog.Handler {
	return &AxiomHandler{
		dataset: h.dataset,
		token:   h.token,
		url:     h.url,
		client:  h.client,
		next:    h.next.WithGroup(name),
		buffer:  h.buffer,
		stop:    h.stop,
	}
}

func (h *AxiomHandler) run() {
	ticker := time.NewTicker(axiomFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = h.Flush()
		case <-h.stop:
			return
		}
	}
}

func (h *AxiomHandler) Flush() error {
	h.mu.Lock()
	if len(h.buffer) == 0 {
		h.mu.Unlock()
		return nil
	}
	batch := h.buffer
	h.buffer = make([]map[string]any, 0, axiomBufferSize)
	h.mu.Unlock()

	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("axiom ingest failed (%d): %s", resp.StatusCode, string(b))
	}

	return nil
}

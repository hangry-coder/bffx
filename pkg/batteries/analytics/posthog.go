package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/hangry-coder/bffx/pkg/logger"
)

func (p *PostHog) Type() string {
	return "posthog"
}

const (
	defaultFlushEvery  = 30 * time.Second
	defaultMaxBatch    = 20
	defaultPostHogHost = "https://app.posthog.com"
	maxRetries         = 4
	initialBackoff     = 100 * time.Millisecond
)

type posthogEvent struct {
	Event      string         `json:"event"`
	Properties map[string]any `json:"properties"`
	Timestamp  string         `json:"timestamp,omitempty"`
	DistinctID string         `json:"distinct_id"`
	UUID       string         `json:"uuid,omitempty"`
	Library    string         `json:"library"`
	LibVersion string         `json:"lib_version"`
}

type batchPayload struct {
	APIKey string         `json:"api_key"`
	Batch  []posthogEvent `json:"batch"`
}

// PostHog batches captures and POSTs them to PostHog's /batch/ endpoint.
type PostHog struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	mu         sync.Mutex
	queue      []posthogEvent
	closed     bool
	flushTimer *time.Timer

	flushMu sync.Mutex
}

// NewPostHog creates a PostHog batching client. apiKey is the project API key.
func NewPostHog(apiKey, host string, hc *http.Client) *PostHog {
	if host == "" {
		host = defaultPostHogHost
	}
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &PostHog{
		apiKey:     apiKey,
		baseURL:    trimRightSlash(host),
		httpClient: hc,
	}
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// Track enqueues an analytics event. Safe for concurrent use.
func (p *PostHog) Track(ctx context.Context, event string, properties map[string]any) {
	if event == "" {
		return
	}
	distinct := "$server"
	if properties != nil {
		if d, ok := properties["distinct_id"].(string); ok && d != "" {
			distinct = d
		}
	}
	props := map[string]any{}
	for k, v := range properties {
		if k == "distinct_id" {
			continue
		}
		props[k] = v
	}
	ev := posthogEvent{
		Event:      event,
		Properties: props,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DistinctID: distinct,
		Library:    "bffx",
		LibVersion: "0.1",
	}

	var batch []posthogEvent
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.queue = append(p.queue, ev)
	n := len(p.queue)
	if n >= defaultMaxBatch {
		batch = p.queue
		p.queue = nil
		if p.flushTimer != nil {
			p.flushTimer.Stop()
			p.flushTimer = nil
		}
	}
	p.mu.Unlock()

	if len(batch) > 0 {
		go func(b []posthogEvent) {
			_ = p.sendWithRetry(ctx, b)
		}(batch)
		return
	}
	// Debounced time-based flush (max defaultFlushEvery after last event).
	p.mu.Lock()
	if !p.closed {
		if p.flushTimer != nil {
			p.flushTimer.Stop()
		}
		p.flushTimer = time.AfterFunc(defaultFlushEvery, func() {
			_ = p.Flush(context.Background())
		})
	}
	p.mu.Unlock()
}

// Flush sends all queued events.
func (p *PostHog) Flush(ctx context.Context) error {
	p.mu.Lock()
	var batch []posthogEvent
	if len(p.queue) > 0 {
		batch = p.queue
		p.queue = nil
	}
	if p.flushTimer != nil {
		p.flushTimer.Stop()
		p.flushTimer = nil
	}
	p.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	return p.sendWithRetry(ctx, batch)
}

func (p *PostHog) sendWithRetry(ctx context.Context, events []posthogEvent) error {
	p.flushMu.Lock()
	defer p.flushMu.Unlock()

	backoff := initialBackoff
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := p.postBatch(ctx, events)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			logger.Warn("posthog: batch send failed (no retry): %v", err)
			return err
		}
		logger.Warn("posthog: batch send failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
	return lastErr
}

func isRetryable(err error) bool {
	var he *posthogHTTPError
	if errors.As(err, &he) {
		return he.status >= 500 || he.status == 429
	}
	return true
}

type posthogHTTPError struct {
	status int
	body   string
}

func (e *posthogHTTPError) Error() string {
	return fmt.Sprintf("posthog http %d: %s", e.status, e.body)
}

func (p *PostHog) postBatch(ctx context.Context, events []posthogEvent) error {
	body, err := json.Marshal(batchPayload{APIKey: p.apiKey, Batch: events})
	if err != nil {
		return err
	}
	url := p.baseURL + "/batch/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &posthogHTTPError{status: resp.StatusCode, body: string(rb)}
	}
	return nil
}

// Close stops delayed flush timers and sends pending events.
func (p *PostHog) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	if p.flushTimer != nil {
		p.flushTimer.Stop()
		p.flushTimer = nil
	}
	rest := p.queue
	p.queue = nil
	p.mu.Unlock()

	var err error
	if len(rest) > 0 {
		err = p.sendWithRetry(ctx, rest)
	}
	if e := p.Flush(ctx); e != nil && err == nil {
		err = e
	}
	return err
}

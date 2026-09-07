package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const stripeSigTolerance = 300 * time.Second

// StripeEventHooks receives verified Stripe webhook events.
// Implement only the callbacks you care about; nil funcs are skipped.
type StripeEventHooks struct {
	OnEvent func(ctx context.Context, eventType string, eventID string, data json.RawMessage) error
}

// StripeWebhook verifies Stripe signatures and dispatches events (idempotent by event id).
type StripeWebhook struct {
	secret  string
	hooks   StripeEventHooks
	backend stripeDedupeBackend
}

type stripeDedupeBackend interface {
	MarkOnce(ctx context.Context, eventID string, ttl time.Duration) (new bool, err error)
}

type memoryDedupe struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func newMemoryDedupe() *memoryDedupe {
	return &memoryDedupe{m: make(map[string]time.Time)}
}

func (m *memoryDedupe) MarkOnce(_ context.Context, eventID string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.m[eventID]; ok {
		return false, nil
	}
	m.m[eventID] = time.Now()
	// crude cap
	if len(m.m) > 8000 {
		m.m = make(map[string]time.Time)
	}
	_ = ttl
	return true, nil
}

type redisDedupe struct {
	r *redis.Client
}

func (r *redisDedupe) MarkOnce(ctx context.Context, eventID string, ttl time.Duration) (bool, error) {
	key := "bffx:stripe:event:" + eventID
	ok, err := r.r.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// NewStripeWebhook constructs a handler. If redis is non-nil, idempotency uses SET NX; otherwise in-memory (single-instance only).
func NewStripeWebhook(secret string, rdb *redis.Client, hooks StripeEventHooks) *StripeWebhook {
	var backend stripeDedupeBackend = newMemoryDedupe()
	if rdb != nil {
		backend = &redisDedupe{r: rdb}
	}
	return &StripeWebhook{
		secret:  secret,
		hooks:   hooks,
		backend: backend,
	}
}

// ServeHTTP expects the raw Stripe JSON body (do not parse JSON before verification).
func (s *StripeWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := VerifyStripeSignature(body, sig, s.secret, stripeSigTolerance); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var envelope struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if envelope.ID == "" {
		http.Error(w, "missing event id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if strictStripeDurableMode() {
		if _, ok := s.backend.(*memoryDedupe); ok {
			http.Error(w, "durable idempotency backend unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	newEvent, err := s.backend.MarkOnce(ctx, envelope.ID, 48*time.Hour)
	if err != nil {
		http.Error(w, "dedupe error", http.StatusInternalServerError)
		return
	}
	if !newEvent {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true,"duplicate":true}`))
		return
	}

	if s.hooks.OnEvent != nil {
		if err := s.hooks.OnEvent(ctx, envelope.Type, envelope.ID, envelope.Data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

func strictStripeDurableMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("BFFX_STRIPE_STRICT_DURABLE_IDEMPOTENCY")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

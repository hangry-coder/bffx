package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/cache/idempotency"
)

type durableStubStore struct {
	mem     *idempotency.MemoryStore
	durable bool
}

func (s *durableStubStore) Get(ctx context.Context, key string) (*idempotency.Response, error) {
	return s.mem.Get(ctx, key)
}

func (s *durableStubStore) Set(ctx context.Context, key string, resp *idempotency.Response, ttl time.Duration) error {
	return s.mem.Set(ctx, key, resp, ttl)
}

func (s *durableStubStore) Durable() bool { return s.durable }

func TestIdempotency(t *testing.T) {
	store := idempotency.NewMemoryStore()

	// Mock handler that returns a specific value
	var callCount int
	handler := Idempotency(store, 1*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-Test", "true")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	// Helper to create request with context
	newReq := func(key string) *http.Request {
		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString("{}"))
		if key != "" {
			req.Header.Set("X-Idempotency-Key", key)
		}
		// Inject mock user claims
		ctx := WithClaims(req.Context(), map[string]any{"sub": "user123"})
		return req.WithContext(ctx)
	}

	// 1. First request (MISS)
	req1 := newReq("key1")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Errorf("1: expected 201, got %d", rr1.Code)
	}
	if rr1.Header().Get("X-BFFX-Idempotency") != "MISS" {
		t.Errorf("1: expected MISS, got %s", rr1.Header().Get("X-BFFX-Idempotency"))
	}
	if callCount != 1 {
		t.Errorf("1: expected handler to be called once, got %d", callCount)
	}

	// 2. Second request with same key (HIT)
	req2 := newReq("key1")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Errorf("2: expected 201, got %d", rr2.Code)
	}
	if rr2.Header().Get("X-BFFX-Idempotency") != "HIT" {
		t.Errorf("2: expected HIT, got %s", rr2.Header().Get("X-BFFX-Idempotency"))
	}
	if rr2.Header().Get("X-Test") != "true" {
		t.Error("2: expected X-Test header to be replayed")
	}
	if rr2.Body.String() != "created" {
		t.Errorf("2: expected 'created', got %s", rr2.Body.String())
	}
	if callCount != 1 {
		t.Errorf("2: expected handler NOT to be called again, callCount=%d", callCount)
	}

	// 3. Request with different key (MISS)
	req3 := newReq("key2")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Header().Get("X-BFFX-Idempotency") != "MISS" {
		t.Errorf("3: expected MISS, got %s", rr3.Header().Get("X-BFFX-Idempotency"))
	}
	if callCount != 2 {
		t.Errorf("3: expected handler to be called again, callCount=%d", callCount)
	}

	// 4. Request without key (Passthrough)
	req4 := newReq("")
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)

	if rr4.Header().Get("X-BFFX-Idempotency") != "" {
		t.Errorf("4: expected no idempotency header, got %s", rr4.Header().Get("X-BFFX-Idempotency"))
	}
	if callCount != 3 {
		t.Errorf("4: expected handler to be called, callCount=%d", callCount)
	}
}

func TestIdempotency_StrictDurableMode(t *testing.T) {
	strict := map[string]bool{"POST /critical": true}

	t.Run("RequiresKeyForStrictRoute", func(t *testing.T) {
		store := &durableStubStore{mem: idempotency.NewMemoryStore(), durable: true}
		handler := IdempotencyWithStrictDurable(store, time.Hour, strict)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/critical", bytes.NewBufferString("{}"))
		req = req.WithContext(WithClaims(req.Context(), map[string]any{"sub": "user123"}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("RequiresDurableBackendForStrictRoute", func(t *testing.T) {
		store := &durableStubStore{mem: idempotency.NewMemoryStore(), durable: false}
		handler := IdempotencyWithStrictDurable(store, time.Hour, strict)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/critical", bytes.NewBufferString("{}"))
		req.Header.Set("X-Idempotency-Key", "k-1")
		req = req.WithContext(WithClaims(req.Context(), map[string]any{"sub": "user123"}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rr.Code)
		}
	})
}

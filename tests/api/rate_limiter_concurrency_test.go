package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
)

// TestRateLimiter_ConcurrentBurst exercises the in-memory token bucket fallback
// in middleware.RateLimit (rdb=nil). With rps=0.1 (slow refill) and burst=5,
// 30 concurrent requests from a single IP should see exactly 5 succeed in the
// burst window and the rest get 429. We use a deterministic burst that's much
// smaller than the worker count so refill noise can't change the count.
func TestRateLimiter_ConcurrentBurst(t *testing.T) {
	const burst = 5
	const workers = 30
	limiter := middleware.RateLimit(nil, 0.1, burst)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var allowed, rejected int32
	var wg sync.WaitGroup
	startGate := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "203.0.113.7:1234"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			switch rr.Code {
			case http.StatusOK:
				atomic.AddInt32(&allowed, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt32(&rejected, 1)
			default:
				t.Errorf("unexpected status %d", rr.Code)
			}
		}()
	}

	close(startGate)
	wg.Wait()

	if got := atomic.LoadInt32(&allowed); got < 1 || got > burst+1 {
		t.Errorf("allowed = %d, want close to burst=%d (1..%d)", got, burst, burst+1)
	}
	if got := atomic.LoadInt32(&rejected); got < workers-burst-1 {
		t.Errorf("rejected = %d, want >= %d", got, workers-burst-1)
	}
	if got := atomic.LoadInt32(&allowed) + atomic.LoadInt32(&rejected); got != workers {
		t.Errorf("allowed+rejected = %d, want %d", got, workers)
	}
}

// TestRateLimiter_DifferentIPsIndependent confirms each IP gets its own bucket.
// Two clients with disjoint IPs should both burst freely.
func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	const burst = 3
	limiter := middleware.RateLimit(nil, 0.1, burst)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	hit := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	for i := 0; i < burst; i++ {
		if code := hit("198.51.100.10"); code != http.StatusOK {
			t.Fatalf("ip A request %d got %d, want 200", i, code)
		}
		if code := hit("198.51.100.20"); code != http.StatusOK {
			t.Fatalf("ip B request %d got %d, want 200", i, code)
		}
	}

	if code := hit("198.51.100.10"); code != http.StatusTooManyRequests {
		t.Errorf("ip A overflow got %d, want 429", code)
	}
	if code := hit("198.51.100.20"); code != http.StatusTooManyRequests {
		t.Errorf("ip B overflow got %d, want 429", code)
	}
}

// TestRateLimiter_ZeroRPSAllowsAll documents the bypass path: rps<=0 means
// the limiter is effectively disabled (used in tests / unrate-limited routes).
func TestRateLimiter_ZeroRPSAllowsAll(t *testing.T) {
	limiter := middleware.RateLimit(nil, 0, 1)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.99:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d got %d, want 200", i, rr.Code)
		}
	}
}

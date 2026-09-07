package middleware

import (
	"fmt"
	"github.com/hangry-coder/bffx/pkg/cache/idempotency"
	"github.com/hangry-coder/bffx/pkg/logger"
	"net/http"
	"time"
)

// Idempotency implements X-Idempotency-Key support.
// It caches successful responses for POST/PUT/PATCH/DELETE requests.
func Idempotency(store idempotency.Store, ttl time.Duration) func(http.Handler) http.Handler {
	return IdempotencyWithStrictDurable(store, ttl, nil)
}

// IdempotencyWithStrictDurable behaves like Idempotency and allows endpoints to
// opt into strict durable mode. Keys in strictRoutes are "METHOD /path".
func IdempotencyWithStrictDurable(store idempotency.Store, ttl time.Duration, strictRoutes map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-Idempotency-Key")
			routeKey := r.Method + " " + r.URL.Path
			strictDurable := strictRoutes != nil && strictRoutes[routeKey]
			// Only apply to mutations
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}
			if strictDurable && key == "" {
				http.Error(w, "x-idempotency-key required for strict durable mode", http.StatusBadRequest)
				return
			}
			if strictDurable && !idempotency.IsDurable(store) {
				http.Error(w, "durable idempotency backend unavailable", http.StatusServiceUnavailable)
				return
			}
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			userID := GetUserID(r.Context())
			if userID == "" {
				userID = "anon"
			}

			// Scope key to user and route to prevent collisions across different actions/users
			fullKey := fmt.Sprintf("%s:%s:%s", userID, r.URL.Path, key)

			if resp, err := store.Get(r.Context(), fullKey); err == nil && resp != nil {
				// Replay headers
				for k, vv := range resp.Headers {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("X-BFFX-Idempotency", "HIT")
				w.WriteHeader(resp.StatusCode)
				// #nosec G705
				w.Write(resp.Body)
				return
			}

			w.Header().Set("X-BFFX-Idempotency", "MISS")
			capture := &ResponseCapture{ResponseWriter: w}
			next.ServeHTTP(capture, r)

			// Cache 2xx and 4xx responses. 5xx should not be cached as they might be transient.
			if capture.Status >= 200 && capture.Status < 500 {
				idempResp := &idempotency.Response{
					StatusCode: capture.Status,
					Headers:    w.Header(),
					Body:       capture.Body,
				}
				if err := store.Set(r.Context(), fullKey, idempResp, ttl); err != nil {
					logger.Warn("Failed to set idempotency cache for %s: %v", fullKey, err)
				}
			}
		})
	}
}

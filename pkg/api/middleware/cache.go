package middleware

import (
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"net/http"
	"time"
)

// ActionCacheKeyPrefix is the key prefix for [Cache] (opt-in action GET response cache).
const ActionCacheKeyPrefix = cache.ActionResponseKeyPrefix

// ActionCacheInvalidatePattern is the pattern for invalidation.
const ActionCacheInvalidatePattern = ActionCacheKeyPrefix + "*"

// CacheOptions configures the GET response cache.
type CacheOptions struct {
	TTL            time.Duration
	VaryHeaders    []string
	AllowAnonymous bool
}

// Cache wraps a handler with battery-backed response caching for `GET` and `HEAD` requests.
func Cache(provider cache.Provider, opts CacheOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if provider == nil || opts.TTL <= 0 || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				w.Header().Set("X-BFFX-Cache", "BYPASS")
				next.ServeHTTP(w, r)
				return
			}

			userID := GetUserID(r.Context())
			if userID == "" && !opts.AllowAnonymous {
				w.Header().Set("X-BFFX-Cache", "BYPASS")
				next.ServeHTTP(w, r)
				return
			}

			key := cacheKey(userID, r, opts.VaryHeaders)

			if val, err := provider.Get(r.Context(), key); err == nil && val != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-BFFX-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				// #nosec G705 -- cached JSON bytes from our own cache store, not request input
				w.Write(val)
				return
			}

			w.Header().Set("X-BFFX-Cache", "MISS")
			capture := &ResponseCapture{ResponseWriter: w}
			next.ServeHTTP(capture, r)

			if capture.Status == http.StatusOK && len(capture.Body) > 0 {
				if err := provider.Set(r.Context(), key, capture.Body, opts.TTL); err != nil {
					logger.Warn("Failed to set cache for %s: %v", key, err)
				}
			}
		})
	}
}

// InvalidateCache deletes one or more cached entries.
func InvalidateCache(ctx context.Context, provider cache.Provider, keys ...string) {
	if provider == nil || len(keys) == 0 {
		return
	}
	for _, key := range keys {
		provider.Delete(ctx, key)
	}
}

func cacheKey(userID string, r *http.Request, vary []string) string {
	return cache.BuildHTTPScopedKey(ActionCacheKeyPrefix, userID, r.Method, r.URL.Path, r.URL.Query(), vary, r.Header.Get)
}

package middleware

import (
	"context"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
)

// IsMutatingHTTPMethod reports whether the HTTP verb may change server-side state.
func IsMutatingHTTPMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// InvalidateActionCache clears all opt-in GET action response cache entries
// (see [Cache] / ActionCacheKeyPrefix). Safe to call after any DB mutation.
func InvalidateActionCache(ctx context.Context, provider cache.Provider) {
	if provider == nil {
		return
	}
	n, err := provider.InvalidatePattern(ctx, ActionCacheInvalidatePattern)
	if err != nil {
		logger.WarnCtx(ctx, "failed to invalidate action response cache: %v", err)
		return
	}
	if n > 0 {
		logger.DebugCtx(ctx, "invalidated %d action cache entries after mutation", n)
	}
}

// InvalidateActionCacheOnMutation wraps the API mux and busts action GET caches after
// any successful POST/PUT/PATCH/DELETE. Complements [storage.InvalidatingStore] tag invalidation.
func InvalidateActionCacheOnMutation(provider cache.Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsMutatingHTTPMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if rec.status >= 200 && rec.status < 300 {
				InvalidateActionCache(r.Context(), provider)
			}
		})
	}
}

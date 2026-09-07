package middleware

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/metrics"
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type anonBucket struct {
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func anonymousRPMFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv("BFFX_ANONYMOUS_AUTH_RPM"))
	if raw == "" {
		return 20
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 20
	}
	return v
}

// AnonymousSessionProtect applies feature-flag disable, metrics, and per-IP RPM limiting
// for POST .../api/v1/auth/anonymous. Mount only on those handlers.
func AnonymousSessionProtect(rdb *redis.Client) func(http.Handler) http.Handler {
	limitPerMinute := anonymousRPMFromEnv()
	rps := float64(limitPerMinute) / 60.0
	burst := int(limitPerMinute)
	if burst < 1 {
		burst = 1
	}
	trust := TrustForwardedHeaders()
	var visitors sync.Map

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if os.Getenv("BFFX_DISABLE_ANONYMOUS_ENDPOINT") == "true" {
				metrics.Global.AnonymousAuthBlocked.Add(1)
				errors.WriteError(w, http.StatusServiceUnavailable, "anonymous sessions disabled")
				return
			}

			ip := ClientIP(r, trust)
			metrics.Global.AnonymousAuthAttempts.Add(1)

			if rdb != nil {
				key := fmt.Sprintf("bffx:ratelimit:anon:%s", ip)
				ctx := r.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				n, err := rdb.Incr(ctx, key).Result()
				if err != nil {
					logger.Warn("Redis anonymous rate limit unavailable: %v", err)
				} else {
					_ = rdb.Expire(ctx, key, time.Minute).Err()
					lim := anonymousRPMFromEnv()
					if lim < 1 {
						lim = 1
					}
					if n > lim {
						metrics.Global.AnonymousAuthRateLimited.Add(1)
						errors.WriteError(w, http.StatusTooManyRequests, "too many anonymous session requests")
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			v, _ := visitors.LoadOrStore(ip, &anonBucket{tokens: float64(burst), last: time.Now()})
			b := v.(*anonBucket)
			b.mu.Lock()
			now := time.Now()
			elapsed := now.Sub(b.last).Seconds()
			b.tokens += elapsed * rps
			if b.tokens > float64(burst) {
				b.tokens = float64(burst)
			}
			b.last = now
			if b.tokens < 1 {
				b.mu.Unlock()
				metrics.Global.AnonymousAuthRateLimited.Add(1)
				errors.WriteError(w, http.StatusTooManyRequests, "too many anonymous session requests")
				return
			}
			b.tokens--
			b.mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

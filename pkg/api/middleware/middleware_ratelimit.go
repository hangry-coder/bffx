package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type bucket struct {
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

// RateLimit provides token-bucket rate limiting, using Redis if available or falling back to in-memory.
func RateLimit(rdb *redis.Client, rps float64, burst int) func(http.Handler) http.Handler {
	var visitors = sync.Map{}

	// Janitor: Clean up expired visitors every 5 minutes to prevent memory leaks
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			visitors.Range(func(key, value any) bool {
				b := value.(*bucket)
				b.mu.Lock()
				// If bucket hasn't been seen for 5 minutes and is full, it's safe to delete
				if time.Since(b.last) > 5*time.Minute && b.tokens >= float64(burst) {
					visitors.Delete(key)
				}
				b.mu.Unlock()
				return true
			})
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lifecycleExemptPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if rps <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			ip := ClientIP(r, TrustForwardedHeaders())

			if rdb != nil {
				key := fmt.Sprintf("bffx:ratelimit:%s", ip)
				pipe := rdb.Pipeline()
				count := pipe.Incr(r.Context(), key)
				pipe.Expire(r.Context(), key, time.Minute)
				_, err := pipe.Exec(r.Context())
				if err != nil {
					logger.WarnCtx(r.Context(), "Redis rate limit unavailable, falling back to memory: %v", err)
				} else {
					limit := int64(rps * 60)
					if limit < 1 {
						limit = 1
					}
					if count.Val() > limit {
						errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			v, _ := visitors.LoadOrStore(ip, &bucket{
				tokens: float64(burst),
				last:   time.Now(),
			})
			b := v.(*bucket)

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
				errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
				return
			}

			b.tokens--
			b.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

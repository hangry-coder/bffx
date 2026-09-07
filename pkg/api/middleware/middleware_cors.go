package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
)

// Timeout returns a middleware that injects a context timeout into the request lifecycle.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORS implements Cross-Origin Resource Sharing based on allowedOrigins and BFFX_CORS_ALLOW_ORIGINS.
// It fails closed in production by default.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allow := false

			if len(allowedOrigins) == 0 {
				envOrigins := os.Getenv("BFFX_CORS_ALLOW_ORIGINS")
				if envOrigins != "" {
					allowedOrigins = strings.Split(envOrigins, ",")
				}
			}

			if len(allowedOrigins) == 0 {
				if os.Getenv("BFFX_ENV") == "production" {
					allow = false // FAIL-CLOSED: same-origin only in production by default
				} else {
					allow = true // Allow in dev for convenience
				}
			} else {
				for _, o := range allowedOrigins {
					o = strings.TrimSpace(o)
					if o == "*" {
						if os.Getenv("BFFX_ENV") == "production" {
							logger.WarnCtx(r.Context(), "CORS: Wildcard origin detected in production; skipping for security.")
							continue
						}
						allow = true
						break
					}
					if o == origin {
						allow = true
						break
					}
				}
			}

			if allow && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if allow && origin == "" && len(allowedOrigins) == 0 && os.Getenv("BFFX_ENV") != "production" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Device-ID, X-App-Secret")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WorkerAuth provides a simple shared-secret authentication mechanism for internal worker nodes.
func WorkerAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				if os.Getenv("BFFX_ENV") == "production" {
					errors.WriteError(w, http.StatusInternalServerError, "worker secret not configured")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteError(w, http.StatusUnauthorized, "worker token required")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if !SecretsEqual(token, secret) {
				errors.WriteError(w, http.StatusUnauthorized, "invalid worker token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

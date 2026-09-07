package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/hangry-coder/bffx/pkg/api/errors"
)

// EnforceAuth is a global policy middleware that rejects any request without valid claims,
// except for documented public health and auth endpoints.
func EnforceAuth(apiPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				path := r.URL.Path
				if path == "/health" || path == "/metrics" || strings.HasPrefix(path, apiPrefix+"/auth") || strings.HasPrefix(path, "/auth") {
					next.ServeHTTP(w, r)
					return
				}
				errors.WriteError(w, http.StatusUnauthorized, "authentication required (global policy)")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func AppSecret(apiPrefix string, secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Mandatory handshake for mobile clients (identified by apiPrefix or /auth prefix)
			if strings.HasPrefix(r.URL.Path, apiPrefix) || strings.HasPrefix(r.URL.Path, "/auth") {
				clientSecret := r.Header.Get("X-App-Secret")
				if !SecretsEqual(clientSecret, secret) {
					errors.WriteError(w, http.StatusForbidden, "invalid app secret")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Admin embeds optional subviews (e.g. legacy docs HTML); SAMEORIGIN allows same-site frames.
		if strings.HasPrefix(r.URL.Path, "/admin") {
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		} else {
			w.Header().Set("X-Frame-Options", "DENY")
		}
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Admin panel serves a React SPA that needs ES module scripts and
		// fetch() calls back to the same origin. Relax the CSP for /admin/*
		// while keeping the strict policy for all other paths.
		if strings.HasPrefix(r.URL.Path, "/admin") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self' data:")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

// MetricsAccess restricts /metrics in production to loopback unless BFFX_ALLOW_PUBLIC_METRICS=true.
func MetricsAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		if os.Getenv("BFFX_ENV") != "production" {
			next.ServeHTTP(w, r)
			return
		}
		if os.Getenv("BFFX_ALLOW_PUBLIC_METRICS") == "true" {
			next.ServeHTTP(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if host != "127.0.0.1" && host != "::1" {
			errors.WriteError(w, http.StatusForbidden, "metrics forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

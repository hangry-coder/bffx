package middleware

import (
	"net/url"
	"net/http"
	"os"
	"strings"
)

// ForceSSL redirects HTTP requests to HTTPS if enabled, checking standard
// proxy headers (like X-Forwarded-Proto). It also ensures Strict-Transport-Security
// headers are set.
func ForceSSL(enabled bool, isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || !isProduction {
				next.ServeHTTP(w, r)
				return
			}

			// In development, or if the request is already HTTPS, bypass redirection.
			// Standard reverse proxies (like Caddy, Nginx, or AWS ALB) set X-Forwarded-Proto.
			isHTTPS := r.TLS != nil ||
				strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https" ||
				strings.ToLower(r.Header.Get("X-Forwarded-SSL")) == "on"

			if !isHTTPS {
				target, ok := redirectTargetFromEnv()
				if !ok {
					http.Error(w, "https redirect host is not configured", http.StatusBadRequest)
					return
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}

			// Enforce HSTS (Strict-Transport-Security) in production
			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}

func redirectTargetFromEnv() (string, bool) {
	host := strings.TrimSpace(os.Getenv("BFFX_PUBLIC_HOST"))
	if host == "" {
		return "", false
	}
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	if host == "" || strings.Contains(host, "/") {
		return "", false
	}

	u := &url.URL{Scheme: "https", Host: host}
	return u.String(), true
}

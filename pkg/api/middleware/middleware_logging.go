package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/observability"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// accessLogMode reads BFFX_HTTP_ACCESS_LOG:
//   - off / false / 0 — no HTTP access logs
//   - on / true / 1 / full / all — log every request (still honors BFFX_HTTP_ACCESS_LOG_SKIP)
//   - minimal (default) — log requests except built-in noisy paths + BFFX_HTTP_ACCESS_LOG_SKIP
func accessLogMode() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("BFFX_HTTP_ACCESS_LOG")))
	switch v {
	case "off", "false", "0", "no", "disabled":
		return "off"
	case "on", "true", "1", "yes", "full", "all", "enabled":
		return "on"
	default:
		return "minimal"
	}
}

func accessLogSkipPatterns() []string {
	raw := strings.TrimSpace(os.Getenv("BFFX_HTTP_ACCESS_LOG_SKIP"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pathMatchesSkipPattern(path, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "/") {
		return path == pattern || strings.HasPrefix(path, pattern+"/")
	}
	return strings.Contains(path, pattern)
}

// defaultSkipAccessLog paths muted in "minimal" mode (health probes, admin SPA polling).
func defaultSkipAccessLog(path string) bool {
	switch path {
	case "/health", "/metrics":
		return true
	}
	if strings.Contains(path, "/admin/metrics") ||
		strings.Contains(path, "/admin/dashboard/data") ||
		strings.Contains(path, "/admin/session") {
		return true
	}
	return strings.HasSuffix(path, "/api/admin/metrics")
}

func shouldAccessLog(path string) bool {
	switch accessLogMode() {
	case "off":
		return false
	case "on":
		// full logging except explicit skip list
	default:
		if defaultSkipAccessLog(path) {
			return false
		}
	}
	for _, pattern := range accessLogSkipPatterns() {
		if pathMatchesSkipPattern(path, pattern) {
			return false
		}
	}
	return true
}

// Logger logs request metadata (method, path, status, duration) and increments global metrics.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		path := r.URL.Path
		if !shouldAccessLog(path) {
			return
		}

		duration := time.Since(start)
		requestID := GetRequestID(r.Context())

		url := path
		if r.URL.RawQuery != "" {
			query := r.URL.Query()
			for _, key := range []string{"password", "token", "secret", "key", "otp"} {
				if query.Get(key) != "" {
					query.Set(key, "[REDACTED]")
				}
			}
			url = url + "?" + query.Encode()
		}

		logger.LogMap("INFO", map[string]any{
			observability.FieldTraceID:   GetTraceID(r.Context()),
			observability.FieldRequestID: requestID,
			"method":                     r.Method,
			"path":                       url,
			"status":                     recorder.status,
			"duration":                   duration.Round(time.Millisecond).String(),
			"ip":                         ClientIP(r, TrustForwardedHeaders()),
		})
	})
}

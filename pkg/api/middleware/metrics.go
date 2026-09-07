package middleware

import (
	"github.com/hangry-coder/bffx/pkg/api/metrics"
	"github.com/hangry-coder/bffx/pkg/observability"
	"net/http"
	"strconv"
	"time"
)

// responseWriter is a wrapper around http.ResponseWriter that captures the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Metrics returns a middleware that records Prometheus metrics for each request.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{w, http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rw.statusCode)
			path := r.URL.Path

			// Record in legacy atomic metrics
			metrics.Global.TotalRequests.Add(1)
			if rw.statusCode >= 400 && rw.statusCode < 500 {
				metrics.Global.Requests4xx.Add(1)
			} else if rw.statusCode >= 500 {
				metrics.Global.Requests5xx.Add(1)
			}

			observability.RecordHTTPRequest(r.Method, path, status, duration)
		})
	}
}

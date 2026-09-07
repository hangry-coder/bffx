package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/metrics"
	"github.com/hangry-coder/bffx/pkg/observability"
	"net/http"
	"runtime"
	"strings"
)

type MetricsHandler struct {
	promHandler http.Handler
}

func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		promHandler: observability.Handler(),
	}
}

func (h *MetricsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	// If the client explicitly asks for JSON, give them the legacy format
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		data := map[string]any{
			"total_requests": metrics.Global.TotalRequests.Load(),
			"requests_4xx":   metrics.Global.Requests4xx.Load(),
			"requests_5xx":   metrics.Global.Requests5xx.Load(),
			"anonymous_auth_attempts":     metrics.Global.AnonymousAuthAttempts.Load(),
			"anonymous_auth_rate_limited": metrics.Global.AnonymousAuthRateLimited.Load(),
			"anonymous_auth_blocked":      metrics.Global.AnonymousAuthBlocked.Load(),
			"goroutines":     runtime.NumGoroutine(),
		}
		errors.WriteJSON(w, http.StatusOK, data)
		return
	}

	// Default to Prometheus text format
	h.promHandler.ServeHTTP(w, r)
}

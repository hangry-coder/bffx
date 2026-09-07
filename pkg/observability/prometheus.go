package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns the standard Prometheus metrics handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

var (
	// HTTPMetrics handles standard API metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricHTTPRequestsTotal,
			Help: "Total number of HTTP requests processed by BFFX.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    MetricHTTPRequestDuration,
			Help:    "Latency of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// AuthMetrics
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricAuthAttemptsTotal,
			Help: "Total number of authentication attempts.",
		},
		[]string{"type", "status"}, // type: signup, login, anonymous, refresh
	)

	// RegistryMetrics
	RegistryResourcesTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricRegistryResourcesTotal,
			Help: "Total number of resources registered in the manifest.",
		},
	)

	// WorkerMetrics
	WorkerJobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricWorkerJobsTotal,
			Help: "Total number of worker jobs processed.",
		},
		[]string{"action", "status"},
	)

	WorkerJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    MetricWorkerJobDuration,
			Help:    "Latency of worker jobs in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"action"},
	)
)

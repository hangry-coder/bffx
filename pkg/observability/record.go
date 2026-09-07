package observability

// RecordHTTPRequest increments canonical HTTP request counters and latency histograms.
func RecordHTTPRequest(method, path, status string, durationSeconds float64) {
	HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(durationSeconds)
}

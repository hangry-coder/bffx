package observability

// Core telemetry semantics for BFFX runtime code and provider adapters.
//
// Two-layer model:
//   - Core semantics (this package): headers, log fields, metric keys, audit shapes.
//   - Provider adapters: pkg/batteries/observability (log shipping), admin dashboard
//     providers in provider.go (incidents/telemetry/analytics read models).

const (
	// HTTP correlation headers (incoming and outgoing).
	HeaderTraceID       = "X-Trace-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderRequestID     = "X-Request-ID"
	HeaderB3TraceID     = "X-B3-TraceId"

	// gRPC metadata keys (lowercase HTTP header equivalents).
	MetadataTraceID       = "x-trace-id"
	MetadataCorrelationID = "x-correlation-id"
	MetadataRequestID     = "x-request-id"
	MetadataTraceParent   = "traceparent"

	// Structured log / JSON error field keys.
	FieldTraceID       = "trace_id"
	FieldRequestID     = "request_id"
	FieldCorrelationID = "correlation_id"
)

const (
	// Prometheus metric names (core semantics; labels defined at registration sites).
	MetricHTTPRequestsTotal       = "bffx_http_requests_total"
	MetricHTTPRequestDuration     = "bffx_http_request_duration_seconds"
	MetricAuthAttemptsTotal       = "bffx_auth_attempts_total"
	MetricRegistryResourcesTotal  = "bffx_registry_resources_total"
	MetricWorkerJobsTotal         = "bffx_worker_jobs_total"
	MetricWorkerJobDuration       = "bffx_worker_job_duration_seconds"
)

const (
	// LogProvider* values for batteries.observability (local-first defaults).
	LogProviderSlog   = "slog"
	LogProviderAxiom  = "axiom"
	LogProviderSentry = "sentry"
)

// Deprecated legacy env selectors that bypass manifest batteries.observability.
// Prefer manifest batteries configuration and canonical provider adapters.
const (
	DeprecatedEnvIncidentProvider  = "BFFX_INCIDENT_PROVIDER"
	DeprecatedEnvTelemetryProvider = "BFFX_TELEMETRY_PROVIDER"
	DeprecatedEnvAnalyticsProvider = "BFFX_ANALYTICS_PROVIDER"
)

// IsCanonicalLogProvider reports whether name is a supported batteries.observability value.
func IsCanonicalLogProvider(name string) bool {
	switch name {
	case LogProviderSlog, LogProviderAxiom, LogProviderSentry:
		return true
	default:
		return false
	}
}

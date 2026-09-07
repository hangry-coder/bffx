package observability

import (
	"net/http"

	"github.com/google/uuid"
)

// ResolveIncomingTraceID picks the canonical trace/correlation ID from request headers.
// Precedence: X-Trace-ID, X-Correlation-ID, X-B3-TraceId.
func ResolveIncomingTraceID(h http.Header) string {
	if h == nil {
		return ""
	}
	if traceID := h.Get(HeaderTraceID); traceID != "" {
		return traceID
	}
	if traceID := h.Get(HeaderCorrelationID); traceID != "" {
		return traceID
	}
	return h.Get(HeaderB3TraceID)
}

// ResolveIncomingRequestID picks the request ID, falling back to traceID when absent.
func ResolveIncomingRequestID(h http.Header, traceID string) string {
	if h != nil {
		if rid := h.Get(HeaderRequestID); rid != "" {
			return rid
		}
	}
	if traceID != "" {
		return traceID
	}
	return uuid.New().String()
}

// EnsureTraceID returns traceID or generates a new UUID when empty.
func EnsureTraceID(traceID string) string {
	if traceID != "" {
		return traceID
	}
	return uuid.New().String()
}

// ApplyCorrelationHeaders writes canonical correlation headers on the response.
// X-Correlation-ID mirrors X-Trace-ID for client compatibility.
func ApplyCorrelationHeaders(w http.ResponseWriter, traceID, requestID string) {
	if w == nil {
		return
	}
	w.Header().Set(HeaderTraceID, traceID)
	w.Header().Set(HeaderRequestID, requestID)
	w.Header().Set(HeaderCorrelationID, traceID)
}

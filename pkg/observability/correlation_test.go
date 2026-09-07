package observability

import (
	"net/http"
	"testing"
)

func TestResolveIncomingTraceIDPrecedence(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderTraceID, "trace-primary")
	h.Set(HeaderCorrelationID, "corr-secondary")
	if got := ResolveIncomingTraceID(h); got != "trace-primary" {
		t.Fatalf("trace header precedence: got %q", got)
	}

	h = http.Header{}
	h.Set(HeaderCorrelationID, "corr-only")
	if got := ResolveIncomingTraceID(h); got != "corr-only" {
		t.Fatalf("correlation fallback: got %q", got)
	}

	h = http.Header{}
	h.Set(HeaderB3TraceID, "b3-only")
	if got := ResolveIncomingTraceID(h); got != "b3-only" {
		t.Fatalf("b3 fallback: got %q", got)
	}
}

func TestApplyCorrelationHeaders(t *testing.T) {
	rr := &headerRecorder{header: make(http.Header)}
	ApplyCorrelationHeaders(rr, "trace-1", "req-1")
	if rr.header.Get(HeaderTraceID) != "trace-1" {
		t.Fatalf("trace header: got %q", rr.header.Get(HeaderTraceID))
	}
	if rr.header.Get(HeaderRequestID) != "req-1" {
		t.Fatalf("request header: got %q", rr.header.Get(HeaderRequestID))
	}
	if rr.header.Get(HeaderCorrelationID) != "trace-1" {
		t.Fatalf("correlation mirrors trace: got %q", rr.header.Get(HeaderCorrelationID))
	}
}

type headerRecorder struct {
	header http.Header
}

func (h *headerRecorder) Header() http.Header { return h.header }
func (h *headerRecorder) Write([]byte) (int, error) { return 0, nil }
func (h *headerRecorder) WriteHeader(int) {}

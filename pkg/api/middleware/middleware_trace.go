package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// RequestID ensures every request has a unique ID, preferring X-Trace-ID or X-Correlation-ID headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := observability.ResolveIncomingTraceID(r.Header)
		traceID = observability.EnsureTraceID(traceID)
		rid := observability.ResolveIncomingRequestID(r.Header, traceID)

		observability.ApplyCorrelationHeaders(w, traceID, rid)

		ctx := r.Context()
		ctx = context.WithValue(ctx, requestIDKey{}, rid)
		ctx = context.WithValue(ctx, traceIDKey{}, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Tracing returns a middleware that starts an OpenTelemetry span for each request.
func Tracing(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			tracer := otel.GetTracerProvider().Tracer("bffx-middleware")
			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.host", r.Host),
				),
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// Inject trace ID into headers for client-side correlation
			if span.SpanContext().HasTraceID() {
				traceID := span.SpanContext().TraceID().String()
				observability.ApplyCorrelationHeaders(w, traceID, traceID)
			}

			// Add IP and UA to context for auditing
			ctx = context.WithValue(ctx, ipKey{}, r.RemoteAddr)
			ctx = context.WithValue(ctx, userAgentKey{}, r.UserAgent())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetIP(ctx context.Context) string {
	if v, ok := ctx.Value(ipKey{}).(string); ok {
		return v
	}
	return ""
}

func GetUserAgent(ctx context.Context) string {
	if v, ok := ctx.Value(userAgentKey{}).(string); ok {
		return v
	}
	return ""
}

func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return "unknown"
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return "unknown"
}

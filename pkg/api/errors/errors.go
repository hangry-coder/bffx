package errors

import (
	core_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/observability"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
)

// APIError represents a standardized error that can be returned to the client.
type APIError struct {
	Status    int            `json:"-"`                    // HTTP status code
	ErrorCode string         `json:"code"`                 // Machine-readable error code
	Message   string         `json:"message"`              // Human-readable message
	Details   map[string]any `json:"details,omitempty"`    // Structured error details
	RequestId string         `json:"request_id,omitempty"` // Unique request ID for tracing
	TraceId   string         `json:"trace_id,omitempty"`   // OpenTelemetry Trace ID
	rawErr    error          `json:"-"`                    // Internal raw error for logging
}

func (e APIError) Error() string {
	return e.Message
}

// WithDetails adds structured details to the error.
func (e APIError) WithDetails(details map[string]any) APIError {
	e.Details = details
	return e
}

// WithRequestID adds a request ID to the error.
func (e APIError) WithRequestID(rid string) APIError {
	e.RequestId = rid
	return e
}

// WithTraceID adds a trace ID to the error.
func (e APIError) WithTraceID(tid string) APIError {
	e.TraceId = tid
	return e
}

// ErrorEnvelope wraps the APIError in a standard JSON envelope.
type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

// PartialResponse is used for 206 Partial Content responses.
type PartialResponse struct {
	Data      any        `json:"data"`
	Errors    []APIError `json:"errors,omitempty"`
	RequestId string     `json:"request_id,omitempty"`
}

// New creates a new APIError with a specific status and machine-readable code.
func New(status int, msg string, errorCode string) APIError {
	return APIError{
		Status:    status,
		Message:   msg,
		ErrorCode: errorCode,
	}
}

// isNetworkOrSysError checks for standard network, context, and system errors and maps them to API responses.
func isNetworkOrSysError(err error) (int, string, string, bool) {
	if err == nil {
		return 0, "", "", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "request timed out", "gateway_timeout", true
	}
	if errors.Is(err, context.Canceled) {
		return 499, "request canceled by client", "client_closed_request", true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return http.StatusBadGateway, "upstream network error occurred", "bad_gateway", true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return http.StatusBadGateway, "failed to resolve upstream address", "bad_gateway", true
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return http.StatusBadGateway, "connection refused by upstream server", "bad_gateway", true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return http.StatusBadGateway, "connection reset by upstream peer", "bad_gateway", true
	}
	if errors.Is(err, syscall.EPIPE) {
		return http.StatusBadGateway, "broken pipe to upstream server", "bad_gateway", true
	}
	if errors.Is(err, syscall.ETIMEDOUT) {
		return http.StatusGatewayTimeout, "upstream network timeout", "gateway_timeout", true
	}

	// Safely retrieve error message for SQLite/DB cancellation string checking
	var errStr string
	func() {
		defer func() { _ = recover() }()
		errStr = strings.ToLower(err.Error())
	}()

	if errStr != "" && (strings.Contains(errStr, "interrupted") || strings.Contains(errStr, "statement cancelled")) {
		return 499, "request canceled by client", "client_closed_request", true
	}

	return 0, "", "", false
}

// FromError converts a generic error into a standardized APIError.
func FromError(err error) APIError {
	if apiErr, ok := err.(APIError); ok {
		if apiErr.rawErr == nil {
			apiErr.rawErr = err
		}
		return apiErr
	}

	type apiErrorConvertible interface {
		ToAPIError() APIError
	}
	if conv, ok := err.(apiErrorConvertible); ok {
		ae := conv.ToAPIError()
		if ae.rawErr == nil {
			ae.rawErr = err
		}
		return ae
	}

	if status, msg, code, ok := isNetworkOrSysError(err); ok {
		ae := New(status, msg, code)
		ae.rawErr = err
		return ae
	}

	if errors.Is(err, core_errors.ErrNotFound) {
		ae := ErrNotFound
		ae.rawErr = err
		return ae
	}
	if errors.Is(err, core_errors.ErrNotImplemented) {
		ae := ErrNotImplemented
		ae.rawErr = err
		return ae
	}
	if errors.Is(err, core_errors.ErrAlreadyExists) {
		ae := New(http.StatusConflict, "resource already exists", "conflict")
		ae.rawErr = err
		return ae
	}

	ae := New(http.StatusInternalServerError, "an unexpected internal server error occurred", "internal_error")
	ae.rawErr = err
	return ae
}

// Write writes a standardized error envelope to the response writer.
func Write(w http.ResponseWriter, err error) {
	apiErr := FromError(err)

	// Extract Request ID and Trace ID from header if set by middleware
	rid := w.Header().Get("X-Request-ID")
	tid := w.Header().Get(observability.HeaderTraceID)
	apiErr.RequestId = rid
	apiErr.TraceId = tid

	raw := apiErr.rawErr
	if raw == nil {
		raw = err
	}
	logger.Error("API HTTP Error Status=%d Code=%s Message=%q RawError=%v RequestID=%s TraceID=%s",
		apiErr.Status, apiErr.ErrorCode, apiErr.Message, raw, rid, tid)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status)

	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: apiErr,
	})
}

// WriteJSON writes a successful JSON response (helper).
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// WritePartial writes a 206 Partial Content response with collected errors.
func WritePartial(w http.ResponseWriter, data any, errs []error) {
	rid := w.Header().Get("X-Request-ID")
	tid := w.Header().Get(observability.HeaderTraceID)

	apiErrs := make([]APIError, 0, len(errs))
	for _, err := range errs {
		apiErr := FromError(err)
		apiErr.RequestId = rid
		apiErr.TraceId = tid
		apiErrs = append(apiErrs, apiErr)

		raw := apiErr.rawErr
		if raw == nil {
			raw = err
		}
		logger.Error("API Partial HTTP Error Status=%d Code=%s Message=%q RawError=%v RequestID=%s TraceID=%s",
			apiErr.Status, apiErr.ErrorCode, apiErr.Message, raw, rid, tid)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPartialContent)
	_ = json.NewEncoder(w).Encode(PartialResponse{
		Data:      data,
		Errors:    apiErrs,
		RequestId: rid,
	})
}

// WriteError is a helper for writing ad-hoc errors without pre-defining them.
func WriteError(w http.ResponseWriter, status int, message string, errorCode ...string) {
	code := "bad_request"
	if len(errorCode) > 0 {
		code = errorCode[0]
	}
	Write(w, New(status, message, code))
}

var (
	ErrNotFound        = New(http.StatusNotFound, "resource not found", "not_found")
	ErrBadRequest      = New(http.StatusBadRequest, "bad request", "bad_request")
	ErrForbidden       = New(http.StatusForbidden, "forbidden", "forbidden")
	ErrUnauthorized    = New(http.StatusUnauthorized, "unauthorized", "unauthorized")
	ErrNotImplemented  = New(http.StatusNotImplemented, "feature not implemented", "not_implemented")
	ErrInternal        = New(http.StatusInternalServerError, "internal server error", "internal_error")
	ErrTooManyRequests = New(http.StatusTooManyRequests, "rate limit exceeded", "rate_limit_exceeded")
	ErrConflict        = New(http.StatusConflict, "resource conflict", "conflict")
	ErrValidation      = New(http.StatusUnprocessableEntity, "validation failed", "validation_error")
)

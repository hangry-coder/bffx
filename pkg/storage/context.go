package storage

import (
	"context"
	"errors"
	"strings"
)

// ContextError reports whether err is (or wraps) a client cancellation or
// deadline exceeded from context propagation. Use it at API boundaries to map
// driver-specific errors to stable behavior (e.g. 499 / 408) without matching
// strings from individual drivers.
func ContextError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Some sqlite builds surface "interrupted" without wrapping std context errors.
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "interrupted") || strings.Contains(s, "statement cancelled") {
		return true
	}
	return false
}

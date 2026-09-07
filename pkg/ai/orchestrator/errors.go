package orchestrator

import (
	"fmt"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
)

// PipelineError represents standard execution failures inside the AI Orchestrator.
type PipelineError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e PipelineError) Error() string {
	return fmt.Sprintf("[%s] %s (retryable: %t)", e.Code, e.Message, e.Retryable)
}

// ToAPIError converts this custom pipeline failure into a standard framework APIError.
func (e PipelineError) ToAPIError() errors.APIError {
	status := http.StatusInternalServerError
	switch e.Code {
	case "QUOTA_EXCEEDED":
		status = http.StatusPaymentRequired
	case "UNAUTHORIZED":
		status = http.StatusUnauthorized
	case "INVALID_ASSET", "VALIDATION_FAILED":
		status = http.StatusBadRequest
	case "RATE_LIMIT_EXCEEDED":
		status = http.StatusTooManyRequests
	case "UPSTREAM_TEMPORARY_FAILURE":
		status = http.StatusServiceUnavailable
	}

	// If marked as retryable but status remains 500, elevate to 503 Service Unavailable
	if e.Retryable && status == http.StatusInternalServerError {
		status = http.StatusServiceUnavailable
	}

	return errors.New(status, e.Message, e.Code)
}

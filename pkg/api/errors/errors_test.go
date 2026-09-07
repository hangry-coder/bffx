package errors

import (
	core_errors "github.com/hangry-coder/bffx/pkg/errors"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		err := New(http.StatusBadRequest, "msg", "type")
		assert.Equal(t, "msg", err.Error())
		assert.Equal(t, http.StatusBadRequest, err.Status)
	})

	t.Run("FromError", func(t *testing.T) {
		// APIError identity
		apiErr := New(http.StatusConflict, "conflict", "type")
		res := FromError(apiErr)
		assert.Equal(t, apiErr.Status, res.Status)
		assert.Equal(t, apiErr.Message, res.Message)
		assert.Equal(t, apiErr.ErrorCode, res.ErrorCode)

		// Core Errors
		resNotFound := FromError(core_errors.ErrNotFound)
		assert.Equal(t, ErrNotFound.Status, resNotFound.Status)
		assert.Equal(t, ErrNotFound.Message, resNotFound.Message)

		resNotImplemented := FromError(core_errors.ErrNotImplemented)
		assert.Equal(t, ErrNotImplemented.Status, resNotImplemented.Status)
		assert.Equal(t, ErrNotImplemented.Message, resNotImplemented.Message)

		// Context Cancellation / Timeouts
		resCanceled := FromError(context.Canceled)
		assert.Equal(t, 499, resCanceled.Status)
		assert.Equal(t, "client_closed_request", resCanceled.ErrorCode)

		resTimeout := FromError(context.DeadlineExceeded)
		assert.Equal(t, http.StatusGatewayTimeout, resTimeout.Status)
		assert.Equal(t, "gateway_timeout", resTimeout.ErrorCode)

		// Interrupted string errors (e.g. Sqlite)
		resInterrupted := FromError(errors.New("query interrupted by user"))
		assert.Equal(t, 499, resInterrupted.Status)
		assert.Equal(t, "client_closed_request", resInterrupted.ErrorCode)

		// System / Network Errors
		resConnRefused := FromError(syscall.ECONNREFUSED)
		assert.Equal(t, http.StatusBadGateway, resConnRefused.Status)
		assert.Equal(t, "bad_gateway", resConnRefused.ErrorCode)

		resConnReset := FromError(syscall.ECONNRESET)
		assert.Equal(t, http.StatusBadGateway, resConnReset.Status)
		assert.Equal(t, "bad_gateway", resConnReset.ErrorCode)

		resPipe := FromError(syscall.EPIPE)
		assert.Equal(t, http.StatusBadGateway, resPipe.Status)
		assert.Equal(t, "bad_gateway", resPipe.ErrorCode)

		resSysTimeout := FromError(syscall.ETIMEDOUT)
		assert.Equal(t, http.StatusGatewayTimeout, resSysTimeout.Status)
		assert.Equal(t, "gateway_timeout", resSysTimeout.ErrorCode)

		resOpErr := FromError(&net.OpError{Op: "dial", Net: "tcp"})
		assert.Equal(t, http.StatusBadGateway, resOpErr.Status)
		assert.Equal(t, "bad_gateway", resOpErr.ErrorCode)

		// Generic Unhandled / Database / Internal Server Error (Sanitization)
		unknown := errors.New("database connection failed: select * from users where secret = 'XYZ'")
		resInternal := FromError(unknown)
		assert.Equal(t, http.StatusInternalServerError, resInternal.Status)
		assert.Equal(t, "an unexpected internal server error occurred", resInternal.Message)
		assert.Equal(t, "internal_error", resInternal.ErrorCode)
	})

	t.Run("Write", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Header().Set("X-Request-ID", "req-123")
		rr.Header().Set("X-Trace-ID", "trace-456")

		Write(rr, ErrNotFound)
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.Contains(t, rr.Body.String(), "req-123")
		assert.Contains(t, rr.Body.String(), "trace-456")
	})

	t.Run("WriteJSON", func(t *testing.T) {
		rr := httptest.NewRecorder()
		WriteJSON(rr, http.StatusCreated, map[string]string{"foo": "bar"})
		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Contains(t, rr.Body.String(), "foo")
	})

	t.Run("WriteError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		WriteError(rr, http.StatusBadRequest, "bad")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "bad")
	})

	t.Run("WritePartial", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Header().Set("X-Request-ID", "req-789")
		WritePartial(rr, "partial_data", []error{errors.New("db fail"), ErrNotFound})
		assert.Equal(t, http.StatusPartialContent, rr.Code)
		assert.Contains(t, rr.Body.String(), "partial_data")
		assert.Contains(t, rr.Body.String(), "req-789")
		assert.Contains(t, rr.Body.String(), "an unexpected internal server error occurred")
	})
}

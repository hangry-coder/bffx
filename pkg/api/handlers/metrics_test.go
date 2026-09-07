package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler(t *testing.T) {
	h := NewMetricsHandler()

	t.Run("GetMetrics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Accept", "application/json")
		rr := httptest.NewRecorder()
		h.GetMetrics(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Contains(t, res, "total_requests")
		assert.Contains(t, res, "goroutines")
	})
}

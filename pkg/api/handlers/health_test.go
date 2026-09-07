package handlers

import (
	"github.com/hangry-coder/bffx/pkg/storage"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
	s := storage.NewMemoryStore()
	h := NewHealthHandler(s, nil)

	t.Run("Health_OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		h.Health(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "ok", res["status"])
		assert.Equal(t, "ok", res["checks"].(map[string]any)["db"])
		assert.Equal(t, "skipped", res["checks"].(map[string]any)["redis"])
	})
}

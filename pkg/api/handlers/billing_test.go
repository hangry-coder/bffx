package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/addons/billing"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestBillingHandler(t *testing.T) {
	s := storage.NewMemoryStore()
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Entitlement"}},
		},
	}
	mgr := billing.NewManager(s, reg)
	h := NewBillingHandler(mgr, s, "apple-secret", "google-config")

	t.Run("Verify Apple Purchase - Success", func(t *testing.T) {
		body := map[string]any{
			"store":          "app_store",
			"product_id":     "premium_tier",
			"transaction_id": "tx_apple_111",
			"receipt_data":   "MOCK_APPLE_RECEIPT_data",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/billing/verify", bytes.NewReader(b))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Verify(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "success", res["status"])
		assert.Equal(t, "tx_apple_111", res["transaction_id"])
	})

	t.Run("Verify Purchase - Replay Attack Blocked", func(t *testing.T) {
		body := map[string]any{
			"store":          "app_store",
			"product_id":     "premium_tier",
			"transaction_id": "tx_apple_111",
			"receipt_data":   "MOCK_APPLE_RECEIPT_data",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/billing/verify", bytes.NewReader(b))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_bob"}))
		rr := httptest.NewRecorder()

		h.Verify(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "transaction_replay", res["error"])
	})

	t.Run("Verify Purchase - Same User Retry Allowed", func(t *testing.T) {
		body := map[string]any{
			"store":          "app_store",
			"product_id":     "premium_tier",
			"transaction_id": "tx_apple_111",
			"receipt_data":   "MOCK_APPLE_RECEIPT_data",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/billing/verify", bytes.NewReader(b))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Verify(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "success", res["status"])
	})

	t.Run("Restore Purchases - Mock Success", func(t *testing.T) {
		body := map[string]any{
			"store":        "app_store",
			"receipt_data": "MOCK_APPLE_RECEIPT_restore",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/billing/restore", bytes.NewReader(b))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Restore(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "success", res["status"])
		assert.Contains(t, res["entitlements"].([]any), "pro_features")
	})

	t.Run("Missing Parameters Error", func(t *testing.T) {
		body := map[string]any{
			"store": "app_store",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/billing/verify", bytes.NewReader(b))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Verify(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "required")
	})
}

func TestVerifyGooglePurchase_Success(t *testing.T) {
	s := storage.NewMemoryStore()
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Entitlement"}},
		},
	}
	mgr := billing.NewManager(s, reg)
	h := NewBillingHandler(mgr, s, "", "")

	body := map[string]any{
		"store":          "play_store",
		"product_id":     "vip_yearly",
		"transaction_id": "tx_google_222",
		"receipt_data":   "MOCK_GOOGLE_RECEIPT_data",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/billing/verify", bytes.NewReader(b))
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_charlie"}))
	rr := httptest.NewRecorder()

	h.Verify(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var res map[string]any
	json.Unmarshal(rr.Body.Bytes(), &res)
	assert.Equal(t, "success", res["status"])
}

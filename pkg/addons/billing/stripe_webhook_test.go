package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func signStripeBody(secret string, body []byte) string {
	ts := time.Now().Unix()
	signed := fmt.Sprintf("%d.%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func TestStripeWebhook_ValidSignatureDispatchesOnce(t *testing.T) {
	secret := "whsec_testsecret"
	var calls atomic.Int32
	h := NewStripeWebhook(secret, nil, StripeEventHooks{
		OnEvent: func(ctx context.Context, eventType, eventID string, data json.RawMessage) error {
			calls.Add(1)
			return nil
		},
	})
	body := []byte(`{"id":"evt_unique_1","type":"ping","data":{"object":{"x":1}}}`)
	hdr := signStripeBody(secret, body)

	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", bytes.NewReader(body))
		req.Header.Set("Stripe-Signature", hdr)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if do() != http.StatusOK {
		t.Fatal("first request failed")
	}
	if do() != http.StatusOK {
		t.Fatal("replay should still 200")
	}
	if calls.Load() != 1 {
		t.Fatalf("handler calls=%d want 1", calls.Load())
	}
}

func TestStripeWebhook_BadSignature400(t *testing.T) {
	secret := "whsec_testsecret"
	h := NewStripeWebhook(secret, nil, StripeEventHooks{})
	body := []byte(`{"id":"evt_x","type":"ping"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", signStripeBody("whsec_other", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestStripeWebhook_StrictDurableModeRequiresNonMemoryBackend(t *testing.T) {
	t.Setenv("BFFX_STRIPE_STRICT_DURABLE_IDEMPOTENCY", "true")
	secret := "whsec_testsecret"
	h := NewStripeWebhook(secret, nil, StripeEventHooks{})
	body := []byte(`{"id":"evt_strict_1","type":"ping","data":{"object":{"x":1}}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/stripe/webhook", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", signStripeBody(secret, body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

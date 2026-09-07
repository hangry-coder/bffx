package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthCredentialRateLimit_LocksAfterFailures(t *testing.T) {
	credentialAttempts.Range(func(key, _ any) bool {
		credentialAttempts.Delete(key)
		return true
	})
	t.Setenv("BFFX_AUTH_LOGIN_MAX_ATTEMPTS", "3")
	t.Setenv("BFFX_AUTH_LOGIN_LOCKOUT_MINUTES", "15")

	handler := AuthCredentialRateLimit(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	body := `{"email":"attacker@example.com","password":"wrong"}`
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after max attempts, got %d body=%s", rr.Code, rr.Body.String())
	}
}

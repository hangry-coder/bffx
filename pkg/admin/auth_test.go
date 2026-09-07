package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// TestAuthHandler_LogoutClearsHttpOnlyCookie guards the fix for the
// "no login screen for new admin panel" report. The session cookie is
// HttpOnly, so JS cannot clear it; logout MUST be performed server-side
// by setting a Max-Age=-1 / Expires-in-the-past cookie with the same
// name, path, and HttpOnly flag.
func TestAuthHandler_LogoutClearsHttpOnlyCookie(t *testing.T) {
	h := NewAuthHandler(storage.NewMemoryStore(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	h.Logout(w, req)

	if got := w.Code; got != http.StatusOK {
		t.Fatalf("logout returned %d, want 200", got)
	}

	setCookies := w.Result().Header.Values("Set-Cookie")
	if len(setCookies) == 0 {
		t.Fatal("logout did not emit a Set-Cookie header")
	}
	var sessionCookie string
	for _, c := range setCookies {
		if strings.HasPrefix(c, SessionCookieName+"=") {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == "" {
		t.Fatalf("logout did not target %s cookie, got %v", SessionCookieName, setCookies)
	}

	// Must be HttpOnly so a malicious script can never re-introduce the
	// cookie value.
	if !strings.Contains(strings.ToLower(sessionCookie), "httponly") {
		t.Errorf("logout cookie must be HttpOnly, got %q", sessionCookie)
	}
	// Must expire immediately. Browsers honour either Max-Age=-1 or an
	// epoch Expires; we ship both.
	if !(strings.Contains(sessionCookie, "Max-Age=0") ||
		strings.Contains(sessionCookie, "Max-Age=-1") ||
		strings.Contains(sessionCookie, "1970")) {
		t.Errorf("logout cookie must expire immediately, got %q", sessionCookie)
	}
	if !strings.Contains(sessionCookie, "Path=/") {
		t.Errorf("logout cookie path must be /, got %q", sessionCookie)
	}
	// Body should be a stable JSON envelope so the React client can
	// confirm success without parsing arbitrary strings.
	if body := strings.TrimSpace(w.Body.String()); body != `{"status":"ok"}` {
		t.Errorf("logout body = %q, want {\"status\":\"ok\"}", body)
	}
}

func TestAuthHandler_Session(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := t.Context()
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, "AdminUser", map[string]any{
		"email":    "admin@example.com",
		"password": hash,
		"role":     "superadmin",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewAuthHandler(store, nil)
	loginBody, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": "secret123"})
	loginW := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	h.Login(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginW.Code)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/admin/session", nil)
	for _, c := range loginW.Result().Cookies() {
		sessionReq.AddCookie(c)
	}
	sessionW := httptest.NewRecorder()
	AuthMiddleware(store, nil, http.HandlerFunc(h.Session)).ServeHTTP(sessionW, sessionReq)
	if sessionW.Code != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionW.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(sessionW.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["authenticated"] != true {
		t.Fatalf("authenticated = %v, want true", body["authenticated"])
	}
	if body["email"] != "admin@example.com" {
		t.Fatalf("email = %v", body["email"])
	}
}

func TestAuthHandler_LoginRejectsNonAdminRole(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := t.Context()
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, "AdminUser", map[string]any{
		"email":    "viewer@example.com",
		"password": hash,
		"role":     "viewer",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewAuthHandler(store, nil)
	body, _ := json.Marshal(map[string]string{"email": "viewer@example.com", "password": "secret123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("login status = %d, want 403", w.Code)
	}
}

func TestAuthHandler_LoginRateLimit(t *testing.T) {
	t.Setenv("BFFX_ENV", "production")
	t.Setenv("BFFX_ADMIN_LOGIN_RATE_LIMIT", "on")
	h := NewAuthHandler(storage.NewMemoryStore(), nil)

	// Perform 5 failed logins (since email doesn't exist, it will fail)
	payload := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrongpassword",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		h.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("attempt %d: expected status 401, got %d", i+1, w.Code)
		}
	}

	// 6th attempt should be rate limited (429)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	h.Login(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 on 6th attempt, got %d", w.Code)
	}

	var res map[string]any
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if msg, ok := res["error"].(string); !ok || !strings.Contains(msg, "Too many login attempts") {
		t.Errorf("expected error message about rate limiting, got: %v", res["error"])
	}
}


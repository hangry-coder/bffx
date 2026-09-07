package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForceSSL(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Disabled", func(t *testing.T) {
		mw := ForceSSL(false, false)(handler)
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Redirect HTTP to HTTPS", func(t *testing.T) {
		t.Setenv("BFFX_PUBLIC_HOST", "example.com")
		mw := ForceSSL(true, true)(handler)
		req := httptest.NewRequest("GET", "http://example.com/test?a=b", nil)
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if loc != "https://example.com" {
			t.Errorf("expected redirect location https://example.com, got %q", loc)
		}
	})

	t.Run("Missing redirect host returns 400", func(t *testing.T) {
		t.Setenv("BFFX_PUBLIC_HOST", "")
		mw := ForceSSL(true, true)(handler)
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("Already HTTPS bypass", func(t *testing.T) {
		mw := ForceSSL(true, true)(handler)
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("HSTS header set in Production", func(t *testing.T) {
		mw := ForceSSL(true, true)(handler)
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)
		hsts := rec.Header().Get("Strict-Transport-Security")
		if hsts != "max-age=31536000; includeSubDomains; preload" {
			t.Errorf("expected HSTS header, got %q", hsts)
		}
	})
}

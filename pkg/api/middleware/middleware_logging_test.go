package middleware

import (
	"testing"
)

func TestShouldAccessLog(t *testing.T) {
	t.Setenv("BFFX_HTTP_ACCESS_LOG_SKIP", "")

	t.Run("off disables all", func(t *testing.T) {
		t.Setenv("BFFX_HTTP_ACCESS_LOG", "off")
		if shouldAccessLog("/api/v1/users") {
			t.Fatal("expected off")
		}
	})

	t.Run("minimal skips health and admin metrics", func(t *testing.T) {
		t.Setenv("BFFX_HTTP_ACCESS_LOG", "minimal")
		if shouldAccessLog("/health") || shouldAccessLog("/admin/api/admin/metrics/summary") {
			t.Fatal("expected minimal skip")
		}
		if !shouldAccessLog("/api/v1/onboarding/save-profile") {
			t.Fatal("expected API path logged in minimal mode")
		}
	})

	t.Run("on logs admin metrics", func(t *testing.T) {
		t.Setenv("BFFX_HTTP_ACCESS_LOG", "on")
		if !shouldAccessLog("/admin/api/admin/metrics/summary") {
			t.Fatal("expected full logging")
		}
	})

	t.Run("custom skip patterns", func(t *testing.T) {
		t.Setenv("BFFX_HTTP_ACCESS_LOG", "on")
		t.Setenv("BFFX_HTTP_ACCESS_LOG_SKIP", "/api/v1/app/bootstrap")
		if shouldAccessLog("/api/v1/app/bootstrap") {
			t.Fatal("expected custom skip")
		}
		if !shouldAccessLog("/api/v1/me") {
			t.Fatal("expected other paths still logged")
		}
	})
}

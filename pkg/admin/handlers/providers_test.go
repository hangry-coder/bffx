package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
)

func TestProvidersHandler(t *testing.T) {
	handler := handlers.NewProvidersHandler(nil)

	t.Run("GetProviders returns default battery provider info when env not set", func(t *testing.T) {
		os.Unsetenv("BFFX_INCIDENT_PROVIDER")
		os.Unsetenv("BFFX_TELEMETRY_PROVIDER")
		os.Unsetenv("BFFX_ANALYTICS_PROVIDER")

		req := httptest.NewRequest("GET", "/api/admin/providers", nil)
		w := httptest.NewRecorder()

		handler.GetProviders(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var info map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		incidentInfo, ok := info["incident"].(map[string]any)
		if !ok {
			t.Fatal("missing incident info")
		}
		if incidentInfo["name"] != "battery" {
			t.Errorf("expected incident provider name to be battery, got %s", incidentInfo["name"])
		}
	})

	t.Run("GetProviders returns custom provider info when env configured", func(t *testing.T) {
		os.Setenv("BFFX_INCIDENT_PROVIDER", "sentry")
		os.Setenv("BFFX_TELEMETRY_PROVIDER", "datadog")
		defer os.Unsetenv("BFFX_INCIDENT_PROVIDER")
		defer os.Unsetenv("BFFX_TELEMETRY_PROVIDER")

		req := httptest.NewRequest("GET", "/api/admin/providers", nil)
		w := httptest.NewRecorder()

		handler.GetProviders(w, req)

		var info map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		inc, ok := info["incident"].(map[string]any)
		if !ok || inc["name"] != "sentry" {
			t.Errorf("expected incident provider to be sentry, got %v", inc)
		}

		telemetry, ok := info["telemetry"].(map[string]any)
		if !ok || telemetry["name"] != "datadog" {
			t.Errorf("expected telemetry provider to be datadog, got %v", telemetry)
		}
	})

	t.Run("GetProviders analytics defaults to battery", func(t *testing.T) {
		os.Unsetenv("BFFX_ANALYTICS_PROVIDER")

		req := httptest.NewRequest("GET", "/api/admin/providers", nil)
		w := httptest.NewRecorder()
		handler.GetProviders(w, req)

		var info map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		analytics, ok := info["analytics"].(map[string]any)
		if !ok {
			t.Fatal("missing analytics info")
		}
		if analytics["name"] != "battery" {
			t.Errorf("expected analytics provider name battery, got %v", analytics["name"])
		}
		if _, hasVendor := analytics["vendor_url"]; hasVendor {
			t.Error("battery analytics should not expose vendor_url")
		}
	})

	t.Run("GetProviders posthog analytics includes vendor_url", func(t *testing.T) {
		os.Setenv("BFFX_ANALYTICS_PROVIDER", "posthog")
		os.Setenv("BFFX_POSTHOG_PROJECT_ID", "proj-test")
		defer os.Unsetenv("BFFX_ANALYTICS_PROVIDER")
		defer os.Unsetenv("BFFX_POSTHOG_PROJECT_ID")

		req := httptest.NewRequest("GET", "/api/admin/providers", nil)
		w := httptest.NewRecorder()
		handler.GetProviders(w, req)

		var info map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		analytics, ok := info["analytics"].(map[string]any)
		if !ok || analytics["name"] != "posthog" {
			t.Fatalf("expected posthog analytics, got %v", info["analytics"])
		}
		vendorURL, _ := analytics["vendor_url"].(string)
		if vendorURL != "https://app.posthog.com/project/proj-test" {
			t.Errorf("unexpected vendor_url: %q", vendorURL)
		}
	})
}

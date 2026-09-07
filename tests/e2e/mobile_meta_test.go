package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestMobileMetaBootstrap(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name: "test-mobile-meta",
		ScaffoldArgs: []string{
			"--admin-email", "admin@example.com",
			"--admin-password", "admin123",
		},
		Port: 8083,
		HostTests: func(t *testing.T, port int) {
			baseUrl := fmt.Sprintf("http://localhost:%d", port)
			t.Logf("DEBUG: BFFX_APP_SECRET in test: %q", os.Getenv("BFFX_APP_SECRET"))

			// 1. Login as admin to get session cookie
			loginURL := baseUrl + "/admin/api/admin/login"
			loginPayload := `{"email": "admin@example.com", "password": "admin123"}`
			resp, err := http.Post(loginURL, "application/json", bytes.NewBufferString(loginPayload))
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("Admin login failed: %v status: %d", err, resp.StatusCode)
			}
			var sessionCookie *http.Cookie
			for _, c := range resp.Cookies() {
				if c.Name == "bffx_admin_session" {
					sessionCookie = c
					break
				}
			}
			if sessionCookie == nil {
				t.Fatalf("bffx_admin_session cookie not found in response")
			}

			// 2. Inject a DB Translation via Admin API
			t.Log("Injecting custom translation via Admin API...")
			transPayload := `{"key": "login_btn", "locale": "en", "value": "Custom Login Label"}`
			req, _ := http.NewRequest("POST", baseUrl+"/admin/api/admin/resources/AppString", bytes.NewBufferString(transPayload))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(sessionCookie)
			resp, err = http.DefaultClient.Do(req)
			if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
				t.Fatalf("failed to inject translation: %v status: %d", err, resp.StatusCode)
			}

			// 3. Inject an AppConfig (Mobile Meta) via Admin API
			t.Log("Injecting custom config via Admin API...")
			configPayload := `{"key": "primary_color", "value": "#FF0000", "type": "color"}`
			req, _ = http.NewRequest("POST", baseUrl+"/admin/api/admin/resources/AppConfig", bytes.NewBufferString(configPayload))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(sessionCookie)
			resp, err = http.DefaultClient.Do(req)
			if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
				t.Fatalf("failed to inject config: %v status: %d", err, resp.StatusCode)
			}

			// 3. Call Bootstrap API
			t.Log("Calling bootstrap...")
			resp, err = http.Get(baseUrl + "/api/v1/app/bootstrap")
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("failed to call bootstrap: %v status: %d", err, resp.StatusCode)
			}
			defer resp.Body.Close()

			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)

			// Verify App Name
			app, _ := result["app"].(map[string]any)
			if app["name"] != "test-mobile-meta" {
				t.Errorf("expected app name test-mobile-meta, got %v", app["name"])
			}

			// Verify Merged Translations
			trans, _ := result["translations"].(map[string]any)
			if trans["login_btn"] != "Custom Login Label" {
				t.Errorf("expected custom DB translation, got %v", trans["login_btn"])
			}
			if trans["home_tab"] != "Dashboard" { // From i18n/en.yaml
				t.Errorf("expected file translation, got %v", trans["home_tab"])
			}

			// Verify AppConfig
			config, _ := result["config"].([]any)
			found := false
			for _, c := range config {
				cm := c.(map[string]any)
				if cm["key"] == "primary_color" && cm["value"] == "#FF0000" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected primary_color in config, but not found")
			}

			t.Log("✨ Mobile Meta Bootstrap Verified Successfully")
		},
		DockerTests: func(t *testing.T, port int) {
			// In Docker, we just verify the app name and default translations since DB is fresh
			baseUrl := fmt.Sprintf("http://localhost:%d", port)
			resp, err := http.Get(baseUrl + "/api/v1/app/bootstrap")
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("failed to call bootstrap in docker: %v", err)
			}
			defer resp.Body.Close()

			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)
			app, _ := result["app"].(map[string]any)
			if app["name"] != "test-mobile-meta" {
				t.Errorf("expected app name test-mobile-meta in docker, got %v", app["name"])
			}
			t.Log("✨ Docker Success")
		},
	})
}

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestSettingsHandler_GetAndUpdate(t *testing.T) {
	store := storage.NewMemoryStore()
	reg := &manifest.Registry{}

	// Seed some initial app config settings
	_, _ = store.Create(context.Background(), "AppConfig", map[string]any{
		"config_key":   "maintenance",
		"config_value": "false",
	})
	_, _ = store.Create(context.Background(), "AppConfig", map[string]any{
		"config_key":   "min_ios_build",
		"config_value": "14.0.1",
	})

	handler := handlers.NewSettingsHandler(store, reg)

	t.Run("GetSettings lists existing settings as key-value pairs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/settings", nil)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var settings map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if settings["maintenance"] != "false" {
			t.Errorf("expected maintenance to be false, got %s", settings["maintenance"])
		}
		if settings["min_ios_build"] != "14.0.1" {
			t.Errorf("expected min_ios_build to be 14.0.1, got %s", settings["min_ios_build"])
		}
	})

	t.Run("UpdateSettings batches configuration keys update or insert", func(t *testing.T) {
		payload := map[string]string{
			"maintenance":   "true",
			"min_ios_build": "15.0.0",
			"new_setting":   "hello",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/admin/settings", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify updates in store
		records, err := store.List(context.Background(), "AppConfig", 100, 0)
		if err != nil {
			t.Fatalf("list error: %v", err)
		}

		res := make(map[string]string)
		for _, rec := range records {
			key, _ := rec["config_key"].(string)
			val, _ := rec["config_value"].(string)
			if key != "" {
				res[key] = val
			}
		}

		if res["maintenance"] != "true" {
			t.Errorf("expected maintenance to be updated to true, got %s", res["maintenance"])
		}
		if res["min_ios_build"] != "15.0.0" {
			t.Errorf("expected min_ios_build to be updated to 15.0.0, got %s", res["min_ios_build"])
		}
		if res["new_setting"] != "hello" {
			t.Errorf("expected new_setting to be inserted as hello, got %s", res["new_setting"])
		}
	})
}

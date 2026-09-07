package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestIncidentsHandler(t *testing.T) {
	store := storage.NewMemoryStore()

	// Seed some unresolved and resolved incidents
	inc1, _ := store.Create(context.Background(), "Incident", map[string]any{
		"title":       "Database CPU Spike",
		"severity":    "Warning",
		"stack_trace": "",
		"resolved":    false,
		"resolved_by": "",
	})
	inc2, _ := store.Create(context.Background(), "Incident", map[string]any{
		"title":       "Panic in user authentication",
		"severity":    "Critical",
		"stack_trace": "panic context trace",
		"resolved":    false,
		"resolved_by": "",
	})
	_, _ = store.Create(context.Background(), "Incident", map[string]any{
		"title":       "Legacy cache warning",
		"severity":    "Info",
		"stack_trace": "",
		"resolved":    true,
		"resolved_by": "admin@internal.io",
	})

	handler := handlers.NewIncidentsHandler(store)

	t.Run("GetIncidents lists only unresolved incidents", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/incidents", nil)
		w := httptest.NewRecorder()

		handler.GetIncidents(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var records []observability.Incident
		if err := json.Unmarshal(w.Body.Bytes(), &records); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(records) != 2 {
			t.Errorf("expected 2 unresolved incidents, got %d", len(records))
		}

		// Ensure no resolved ones are listed
		for _, r := range records {
			if r.Resolved == true {
				t.Errorf("listed incident %v should not be resolved", r.Title)
			}
		}
	})

	t.Run("ResolveIncident marks an unresolved incident as resolved", func(t *testing.T) {
		id, _ := inc1["id"].(string)
		req := httptest.NewRequest("POST", "/api/admin/incidents/"+id+"/resolve", nil)
		req.SetPathValue("id", id)

		ctx := context.WithValue(req.Context(), "admin_id", "test-admin")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.ResolveIncident(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify status in DB
		updated, err := store.Get(context.Background(), "Incident", id)
		if err != nil {
			t.Fatalf("failed to retrieve updated incident: %v", err)
		}

		if updated["resolved"] != true {
			t.Errorf("expected resolved to be true, got %v", updated["resolved"])
		}
		if updated["resolved_by"] != "test-admin" {
			t.Errorf("expected resolved_by to be test-admin, got %v", updated["resolved_by"])
		}
	})

	t.Run("GetIncidents returns empty list after resolving remaining incident", func(t *testing.T) {
		id2, _ := inc2["id"].(string)
		req := httptest.NewRequest("POST", "/api/admin/incidents/"+id2+"/resolve", nil)
		req.SetPathValue("id", id2)
		w := httptest.NewRecorder()

		handler.ResolveIncident(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Check unresolved list again
		reqList := httptest.NewRequest("GET", "/api/admin/incidents", nil)
		wList := httptest.NewRecorder()
		handler.GetIncidents(wList, reqList)

		var records []observability.Incident
		if err := json.Unmarshal(wList.Body.Bytes(), &records); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if len(records) != 0 {
			t.Errorf("expected 0 unresolved incidents left, got %d", len(records))
		}
	})
}

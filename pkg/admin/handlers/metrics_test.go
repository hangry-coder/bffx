package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func TestMetricsHandler_GetDashboardData(t *testing.T) {
	// 1. Create a dashboard spec
	var node yaml.Node
	yaml.Unmarshal([]byte(`
widgets:
  - type: metric
    title: "Total Users"
    query:
      resource: User
      aggregate: count
  - type: table
    title: "Recent Users"
    query:
      resource: User
    columns: [name]
    limit: 2
`), &node)
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	dashManifest := &manifest.Manifest{
		Kind:     "AdminDashboard",
		Metadata: manifest.Metadata{Name: "main"},
		Spec:     node,
	}

	userManifest := newResourceManifest(t, "User", "mobile", "mobile")

	reg := &manifest.Registry{
		Resources:       []*manifest.Manifest{userManifest},
		AdminDashboards: []*manifest.Manifest{dashManifest},
	}

	store := storage.NewMemoryStore()
	ctx := context.Background()
	_, _ = store.Create(ctx, "User", map[string]any{"name": "Alice"})
	_, _ = store.Create(ctx, "User", map[string]any{"name": "Bob"})

	h := NewMetricsHandler(nil, store, reg)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/data", nil)
	w := httptest.NewRecorder()
	h.GetDashboardData(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var res struct {
		Widgets []struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			Data  any    `json:"data"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(res.Widgets) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(res.Widgets))
	}

	// Verify metric count widget
	if res.Widgets[0].Type != "metric" || res.Widgets[0].Title != "Total Users" {
		t.Errorf("unexpected metric widget: %+v", res.Widgets[0])
	}
	// The counts are returned as float64 due to JSON unmarshaling
	if val, ok := res.Widgets[0].Data.(float64); !ok || val != 2 {
		t.Errorf("expected count 2, got %v", res.Widgets[0].Data)
	}

	// Verify table widget
	if res.Widgets[1].Type != "table" || res.Widgets[1].Title != "Recent Users" {
		t.Errorf("unexpected table widget: %+v", res.Widgets[1])
	}
	tableData, ok := res.Widgets[1].Data.([]any)
	if !ok || len(tableData) != 2 {
		t.Fatalf("expected table data to have 2 items, got %v", res.Widgets[1].Data)
	}
	r0 := tableData[0].(map[string]any)
	if name, ok := r0["name"].(string); !ok || (name != "Alice" && name != "Bob") {
		t.Errorf("unexpected row name value: %v", r0["name"])
	}
}

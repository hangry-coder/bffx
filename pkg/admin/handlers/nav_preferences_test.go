package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestNavPreferencesHandler_PutResourcePin(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".bffx"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{Root: root}

	h := NewNavPreferencesHandler(reg)
	body, _ := json.Marshal(map[string]any{
		"kind": "resource",
		"name": "Device",
		"pin":  true,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/nav-preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Put(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	h.Get(w2, httptest.NewRequest(http.MethodGet, "/api/admin/nav-preferences", nil))
	var prefs map[string]map[string]bool
	if err := json.Unmarshal(w2.Body.Bytes(), &prefs); err != nil {
		t.Fatal(err)
	}
	if !prefs["resources"]["Device"] {
		t.Fatalf("expected Device pinned, got %v", prefs["resources"])
	}
}

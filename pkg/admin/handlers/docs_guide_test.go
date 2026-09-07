package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/docsbundle"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestDocsHandler_GuideDocs(t *testing.T) {
	root := t.TempDir()
	docsDir := docsbundle.ProjectDocsDir(root)
	if err := os.MkdirAll(filepath.Join(docsDir, "getting-started"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "# Quickstart\n\nBody."
	if err := os.WriteFile(filepath.Join(docsDir, "getting-started", "quickstart.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, _ := json.Marshal(docsbundle.Index{
		Entries: []docsbundle.IndexEntry{{ID: "getting-started/quickstart", Title: "Quickstart", Path: "getting-started/quickstart.md"}},
	})
	if err := os.WriteFile(filepath.Join(docsDir, "index.json"), idx, 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("BFFX_ENV", "development")
	h := NewDocsHandler(&manifest.Registry{Root: root})

	w := httptest.NewRecorder()
	h.ListGuideDocs(w, httptest.NewRequest(http.MethodGet, "/api/admin/docs/guide/index.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("index status = %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/docs/guide/getting-started/quickstart.md", nil)
	req.SetPathValue("path", "getting-started/quickstart.md")
	h.GetGuideDoc(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("doc status = %d body=%s", w2.Code, w2.Body.String())
	}
}

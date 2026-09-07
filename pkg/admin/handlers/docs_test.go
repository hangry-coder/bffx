package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestDocsHandler_GetDocs(t *testing.T) {
	// Setup Registry
	reg := &manifest.Registry{
		Root: t.TempDir(),
	}

	// 1. Production Mode
	os.Setenv("BFFX_ENV", "production")
	h := NewDocsHandler(reg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/docs", nil)
	h.GetDocsHTML(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("production mode GET /api/admin/docs: status = %d, want 404", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/docs/openapi.json", nil)
	h.GetOpenAPISpec(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("production mode GET /api/admin/docs/openapi.json: status = %d, want 404", w.Code)
	}

	// 2. Development Mode
	os.Setenv("BFFX_ENV", "development")

	// Verify HTML response
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/docs", nil)
	h.GetDocsHTML(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("development mode GET /api/admin/docs: status = %d, want 200", w.Code)
	}

	// 3. Optional via features toggle
	// If api_docs is false in site manifest, it should be disabled
	siteYaml := &manifest.Manifest{
		Kind:     "AdminSite",
		Metadata: manifest.Metadata{Name: "default"},
		Spec:     *newYAMLNode(t, "features:\n  api_docs: false\n  app_features: true"),
	}
	reg.AdminSites = []*manifest.Manifest{siteYaml}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/docs", nil)
	h.GetDocsHTML(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("disabled features.api_docs GET /api/admin/docs: status = %d, want 404", w.Code)
	}
}

func TestGetConfig_HidesApiDocsInProduction(t *testing.T) {
	reg := &manifest.Registry{
		Root: t.TempDir(),
	}

	// Create and register a mock AdminSite
	siteYaml := &manifest.Manifest{
		Kind:     "AdminSite",
		Metadata: manifest.Metadata{Name: "default"},
		Spec:     *newYAMLNode(t, "features:\n  api_docs: true"),
	}
	reg.AdminSites = []*manifest.Manifest{siteYaml}

	h := NewResourceHandler(nil, nil, reg, nil)

	// Development: api_docs is true
	os.Setenv("BFFX_ENV", "development")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("development status = %d, want 200", w.Code)
	}

	var graph manifest.AdminGraph
	if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !graph.Site.Features.ApiDocs {
		t.Errorf("expected api_docs to be true in development")
	}

	// Production: api_docs is forced to false
	os.Setenv("BFFX_ENV", "production")
	w = httptest.NewRecorder()
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("production status = %d, want 200", w.Code)
	}
	graph = manifest.AdminGraph{}
	if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
		t.Fatalf("json: %v", err)
	}
	if graph.Site.Features.ApiDocs {
		t.Errorf("expected api_docs to be false in production")
	}
}

func newYAMLNode(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return &node
}

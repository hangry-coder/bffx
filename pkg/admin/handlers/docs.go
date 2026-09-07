package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

type DocsHandler struct {
	reg *manifest.Registry
}

func NewDocsHandler(reg *manifest.Registry) *DocsHandler {
	return &DocsHandler{reg: reg}
}

func (h *DocsHandler) isEnabled() bool {
	if os.Getenv("BFFX_ENV") == "production" {
		return false
	}

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err != nil {
		return false
	}

	// If site.yaml is present and has api_docs specifically set, respect it.
	// Otherwise, it defaults to true in non-production.
	return graph.Site.Features.ApiDocs
}

// GetOpenAPISpec returns the openapi.json file content if allowed
func (h *DocsHandler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if !h.isEnabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	specPath := filepath.Join(h.reg.Root, ".bffx", "openapi.json")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, specPath)
}

// GetDocsHTML returns the Swagger UI HTML page if allowed
func (h *DocsHandler) GetDocsHTML(w http.ResponseWriter, r *http.Request) {
	if !h.isEnabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>BFFX Swagger</title>
  <link rel="stylesheet" href="/admin/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="/admin/swagger-ui-bundle.js" crossorigin></script>
<script>
  window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: (window.location.pathname.replace(/\/?$/, "/") + "openapi.json"),
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
  };
</script>
</body>
</html>`))
}

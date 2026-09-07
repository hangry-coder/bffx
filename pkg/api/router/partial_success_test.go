package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestScreens_PartialSuccess_206(t *testing.T) {
	var spec yaml.Node
	if err := yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/home
partial_success: true
sources:
  - app
  - currentUser
output:
  appName: app.name
  user: currentUser
`), &spec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Path:     "/tmp/bffx/project.yaml",
		},
		Screens: []*manifest.Manifest{{Kind: "Screen", Metadata: manifest.Metadata{Name: "Home"}, Spec: spec}},
	}

	r := &Router{
		reg:   reg,
		mux:   http.NewServeMux(),
		store: storage.NewMemoryStore(),
	}
	r.registerScreenRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/screens/home", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "missing-user"}))
	rr := httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPartialContent, rr.Code)

	var body struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "TestApp", body.Data["appName"])
	assert.NotEmpty(t, body.Errors)
}

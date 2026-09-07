package router

import (
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestActions_RegisterAndExecute(t *testing.T) {
	var spec yaml.Node
	yaml.Unmarshal([]byte(`
route:
  method: POST
  path: /api/v1/actions/test
  auth: required
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Path:     "/tmp/bffx/project.yaml",
		},
		Actions: []*manifest.Manifest{
			{
				Kind:     "Action",
				Metadata: manifest.Metadata{Name: "TestAction"},
				Spec:     spec,
			},
		},
	}

	r := &Router{
		reg:   reg,
		mux:   http.NewServeMux(),
		store: storage.NewMemoryStore(),
		actionHandlers: map[string]handlers.ActionHandler{
			"TestAction": func(ctx *handlers.ActionContext, w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("action executed"))
			},
		},
	}

	r.registerActionRoutes()

	// Test Unauthenticated
	req := httptest.NewRequest("POST", "/api/v1/actions/test", nil)
	rr := httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Test Authenticated
	req = httptest.NewRequest("POST", "/api/v1/actions/test", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user-1"}))
	rr = httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "action executed", rr.Body.String())
}

func TestActions_WrapWithAuth(t *testing.T) {
	r := &Router{}
	handler := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "required")

	// Missing claims -> 401
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// With claims -> 200
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "u1"}))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStrictDurableIdempotencyRoutes(t *testing.T) {
	var spec yaml.Node
	yaml.Unmarshal([]byte(`
route:
  method: POST
  path: /api/v1/actions/critical
  auth: required
  idempotency_mode: durable
`), &spec)
	reg := &manifest.Registry{
		Actions: []*manifest.Manifest{
			{
				Kind:     "Action",
				Metadata: manifest.Metadata{Name: "CriticalAction"},
				Spec:     spec,
			},
		},
	}
	r := &Router{reg: reg}
	got := r.strictDurableIdempotencyRoutes()
	if !got["POST /api/v1/actions/critical"] {
		t.Fatalf("expected durable route to be registered, got %#v", got)
	}
}

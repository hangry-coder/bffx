package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRouterAuthAndCRUD(t *testing.T) {
	s := storage.NewMemoryStore()

	var userSpec, noteSpec yaml.Node
	yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: email, type: string, required: true}
  - {name: password, type: string, required: true}
`), &userSpec)
	yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: text, type: string, required: true}
`), &noteSpec)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "User"},
				Spec:     userSpec,
			},
			{
				Metadata: manifest.Metadata{Name: "Note"},
				Spec:     noteSpec,
			},
		},
	}

	bus := events.NewMemoryBus()
	notify := notifications.NewManager(s)
	emailMgr := email.NewManager()
	hub := comm.NewHub(s, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")

	flagProvider := providers.NewBffxProvider(s, reg)
	jwt := auth.NewJWTService("test-secret")
	r := NewRouter(RouterConfig{
		Store:         s,
		Telemetry:     s,
		JobStore:      worker.NewMemoryJobStore(),
		Registry:      reg,
		AuthProvider:  jwt,
		JWTService:    jwt,
		EventBus:      bus,
		Notifications: notify,
		Email:         emailMgr,
		CommHub:       hub,
		I18n:          bundle,
		FlagProvider:  flagProvider,
	})
	handler := r.Setup()

	signupBody := `{"email": "test@example.com", "password": "password123", "name": "Test User"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewBufferString(signupBody))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("signup failed: %v", rr.Body.String())
	}

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %v", rr.Body.String())
	}

	var loginResp struct{ Token string }
	json.Unmarshal(rr.Body.Bytes(), &loginResp)
	token := loginResp.Token

	req = httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /me failed: %v", rr.Body.String())
	}

	noteBody := `{"text": "Hello World"}`
	req = httptest.NewRequest("POST", "/api/v1/notes", bytes.NewBufferString(noteBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/notes failed: %v", rr.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/notes", bytes.NewBufferString(noteBody))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected unauthorized for unauthenticated request, got %d", rr.Code)
	}

	// Test Health
	req = httptest.NewRequest("GET", "/health", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Test Metrics (Access restricted in production, but here it's dev/test)
	req = httptest.NewRequest("GET", "/metrics", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	t.Run("Pagination_Clamping", func(t *testing.T) {
		ctx := context.Background()
		for i := 0; i < 104; i++ {
			_, err := s.Create(ctx, "Note", map[string]any{"text": fmt.Sprintf("bulk-%d", i)})
			if err != nil {
				t.Fatalf("seed note: %v", err)
			}
		}
		req = httptest.NewRequest("GET", "/api/v1/notes?limit=2000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var list struct {
			Items []map[string]any `json:"items"`
		}
		assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
		assert.Len(t, list.Items, 100, "limit=2000 must clamp to maxCRUDListLimit")
	})

}

func TestRouter_LiveOpsBootstrap(t *testing.T) {
	s := storage.NewMemoryStore()

	// Parse mock LiveOpsEvent manifest
	var m manifest.Manifest
	err := yaml.Unmarshal([]byte(`
apiVersion: bffx.io/v1alpha1
kind: LiveOpsEvent
metadata:
  name: double_xp_weekend
spec:
  title: "Double XP Weekend"
  description: "Earn double rewards."
  priority: 25
  audience_segment: "all"
  schedule:
    start_time: "2026-05-01T00:00:00Z"
    end_time: "2036-06-01T00:00:00Z"
  configuration_payload:
    xp_multiplier: 2.0
`), &m)
	require.NoError(t, err)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		LiveOpsEvents: []*manifest.Manifest{&m},
	}

	scheduler := liveops.NewScheduler(s, reg)
	err = scheduler.Init()
	require.NoError(t, err)

	evs, _ := scheduler.ListEvents()
	t.Logf("LIST EVENTS: %+v", evs)

	jwt := auth.NewJWTService("test-secret")
	r := NewRouter(RouterConfig{
		Store:            s,
		Telemetry:        s,
		JobStore:         worker.NewMemoryJobStore(),
		Registry:         reg,
		AuthProvider:     jwt,
		JWTService:       jwt,
		I18n:             i18n.NewBundle("en"),
		FlagProvider:     providers.NewBffxProvider(s, reg),
		LiveOpsScheduler: scheduler,
	})
	handler := r.Setup()

	req := httptest.NewRequest("GET", "/api/v1/app/bootstrap", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var res map[string]any
	json.Unmarshal(rr.Body.Bytes(), &res)

	features, ok := res["features"].(map[string]any)
	require.True(t, ok)

	liveopsList, ok := features["liveops"].([]any)
	require.True(t, ok)
	require.Len(t, liveopsList, 1)

	evt := liveopsList[0].(map[string]any)
	assert.Equal(t, "double_xp_weekend", evt["name"])
	assert.Equal(t, "Double XP Weekend", evt["title"])
	assert.Equal(t, float64(25), evt["priority"])
}

func TestRouter_PublicAPIDocsNotRegistered(t *testing.T) {
	s := storage.NewMemoryStore()
	var userSpec yaml.Node
	yaml.Unmarshal([]byte(`routes:
  crud: true
fields:
  - {name: email, type: string}
`), &userSpec)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{{
			Metadata: manifest.Metadata{Name: "User"},
			Spec:     userSpec,
		}},
	}

	jwt := auth.NewJWTService("test-secret")
	r := NewRouter(RouterConfig{
		Store:        s,
		Telemetry:    s,
		JobStore:     worker.NewMemoryJobStore(),
		Registry:     reg,
		AuthProvider: jwt,
		JWTService:   jwt,
		I18n:         i18n.NewBundle("en"),
		FlagProvider: providers.NewBffxProvider(s, reg),
	})
	handler := r.Setup()

	for _, path := range []string{"/api/docs", "/api/docs/openapi.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code, "path %s should not be publicly served", path)
	}
}

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/router"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
	"context"

	"github.com/hangry-coder/bffx/pkg/auth"
	"gopkg.in/yaml.v3"
)

// buildAuthedNoteRouter returns a router + auth token wired against an
// in-memory store with a single User and a Note resource that has CRUD enabled
// and policy=authenticated. Used by the PATCH-claim-strip and admin-RBAC tests.
func buildRouterWithNote(t *testing.T, writePolicy any) (http.Handler, *storage.MemoryStore, string) {
	t.Helper()
	st := storage.NewMemoryStore()

	var userSpec, noteSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: email, type: string, required: true}
  - {name: password, type: string, required: true}
  - {name: role, type: string}
`), &userSpec); err != nil {
		t.Fatalf("user spec: %v", err)
	}

	noteWritePolicyYAML := "authenticated"
	if s, ok := writePolicy.(string); ok && s != "" {
		noteWritePolicyYAML = s
	}

	noteYAML := `routes:
  crud: true
policy:
  read: authenticated
  write: ` + noteWritePolicyYAML + `
fields:
  - {name: text, type: string, required: true}
  - {name: created_by, type: string}
`
	if err := yaml.Unmarshal([]byte(noteYAML), &noteSpec); err != nil {
		t.Fatalf("note spec: %v", err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "Note"}, Spec: noteSpec},
		},
	}

	bus := events.NewMemoryBus()
	notify := notifications.NewManager(st)
	emailMgr := email.NewManager()
	hub := comm.NewHub(st, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(st, reg)

	jwtSvc := auth.NewJWTService("test-secret")
	r := router.NewRouter(router.RouterConfig{
		Store:         st,
		Telemetry:     st,
		JobStore:      worker.NewMemoryJobStore(),
		Registry:      reg,
		AuthProvider:  auth.NewJWTProvider(jwtSvc),
		JWTService:    jwtSvc,
		EventBus:      bus,
		Notifications: notify,
		Email:         emailMgr,
		CommHub:       hub,
		I18n:          bundle,
		FlagProvider:  flagProvider,
	})
	handler := r.Setup()

	// Sign up + log in to get a token.
	signup := `{"email":"alice@example.com","password":"password123","name":"Alice"}`
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(signup)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rr.Code, rr.Body.String())
	}

	login := `{"email":"alice@example.com","password":"password123"}`
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(login)))
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rr.Code, rr.Body.String())
	}
	var loginResp struct{ Token string }
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil || loginResp.Token == "" {
		t.Fatalf("login response missing token: %v body=%s", err, rr.Body.String())
	}

	return handler, st, loginResp.Token
}

// TestPatch_StripsIDAndCreatedBy asserts that the CRUD PATCH path drops `id`
// and `created_by` from the payload before persisting (router.go:427-428),
// preventing a client from rebinding ownership or hijacking the PK.
func TestPatch_StripsIDAndCreatedBy(t *testing.T) {
	handler, st, token := buildRouterWithNote(t, "authenticated")

	// Create the note via the API so created_by is set by the server.
	createBody := `{"text":"original","created_by":"alice"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewBufferString(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", rr.Code, rr.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	originalID, _ := created["id"].(string)
	if originalID == "" {
		t.Fatalf("created note missing id: %v", created)
	}
	originalCreatedBy, _ := created["created_by"].(string)

	// PATCH with hostile id + created_by overrides.
	patch := map[string]any{
		"id":         "hijacked-id",
		"created_by": "evil-user",
		"text":       "updated",
	}
	body, _ := json.Marshal(patch)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/notes/"+originalID, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", rr.Code, rr.Body.String())
	}

	stored, err := st.Get(context.Background(), "Note", originalID)
	if err != nil {
		t.Fatalf("note %q vanished after PATCH: %v", originalID, err)
	}
	if id, _ := stored["id"].(string); id != originalID {
		t.Errorf("id was rebound: got %q, want %q", id, originalID)
	}
	if cb, _ := stored["created_by"].(string); cb != originalCreatedBy {
		t.Errorf("created_by was rebound: got %q, want %q", cb, originalCreatedBy)
	}
	if text, _ := stored["text"].(string); text != "updated" {
		t.Errorf("text not updated: got %q", text)
	}

	// Hijacked id must not exist.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notes/hijacked-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("hijacked-id should 404, got %d", rr.Code)
	}
}

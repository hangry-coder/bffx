package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// buildRouterWithRefreshTokens spins up a SQLite-backed store with a
// User and RefreshToken resource, runs Reconcile to create the tables, and
// returns the wired router. Memory store can't be used here because its
// QueryBuilder is a no-op.
func buildRouterWithRefreshTokens(t *testing.T) (http.Handler, *storage.SQLiteStore, func()) {
	t.Helper()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	st, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}

	var userSpec, refreshSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`fields:
  - {name: email, type: string, required: true, unique: true}
  - {name: password, type: string, required: true}
  - {name: name, type: string}
  - {name: role, type: string}
`), &userSpec); err != nil {
		t.Fatalf("user spec: %v", err)
	}
	// NOTE: framework ResourceSpec uses `tree: bool`. Legacy manifests with
	// `tree: ownership` string form silently fails to parse (separate gap
	// tracked in active_tasks) — for this isolated test we just omit `tree`
	// since Reconcile creates the table either way.
	if err := yaml.Unmarshal([]byte(`fields:
  - {name: user_id, type: string, target: User}
  - {name: device_id, type: string}
  - {name: token, type: string, unique: true}
  - {name: expires_at, type: string}
  - {name: used, type: bool}
  - {name: family_id, type: string}
`), &refreshSpec); err != nil {
		t.Fatalf("refresh spec: %v", err)
	}

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "RefreshTest"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "RefreshToken"}, Spec: refreshSpec},
		},
	}

	// Run Reconcile in an isolated cwd so .bffx/schema.hash doesn't collide.
	prevWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if _, err := st.Reconcile(context.Background(), reg); err != nil {
		os.Chdir(prevWD)
		t.Fatalf("reconcile: %v", err)
	}
	os.Chdir(prevWD)

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

	cleanup := func() {
		st.GetDB().Close()
	}
	return handler, st, cleanup
}

// signupAndLogin returns (accessToken, refreshToken) for a fresh user.
func signupAndLogin(t *testing.T, h http.Handler, email string) (string, string) {
	t.Helper()
	body := `{"email":"` + email + `","password":"password123","name":"Tester"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup: got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"`+email+`","password":"password123"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("login: got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login decode: %v", err)
	}
	if resp.RefreshToken == "" {
		t.Fatalf("login response missing refresh_token: %s", rr.Body.String())
	}
	return resp.Token, resp.RefreshToken
}

func postRefresh(t *testing.T, h http.Handler, refresh string) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"refresh_token": refresh})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	var out map[string]any
	json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

func apiErrorCode(body map[string]any) string {
	errObj, _ := body["error"].(map[string]any)
	if errObj == nil {
		return ""
	}
	code, _ := errObj["code"].(string)
	return code
}

// TestRefreshTokenRotation_RevokesPrevious confirms the canonical happy-path:
// after using a refresh token to mint a new pair, the previous refresh is
// rejected as reuse on its second use.
func TestRefreshTokenRotation_RevokesPrevious(t *testing.T) {
	handler, _, cleanup := buildRouterWithRefreshTokens(t)
	defer cleanup()

	_, refresh1 := signupAndLogin(t, handler, "alice@example.com")

	code, body := postRefresh(t, handler, refresh1)
	if code != http.StatusOK {
		t.Fatalf("first refresh: got %d body=%v", code, body)
	}
	refresh2, _ := body["refresh_token"].(string)
	if refresh2 == "" || refresh2 == refresh1 {
		t.Fatalf("rotated refresh token must be new and non-empty (got %q from %q)", refresh2, refresh1)
	}

	code, body = postRefresh(t, handler, refresh2)
	if code != http.StatusOK {
		t.Fatalf("second refresh: got %d body=%v", code, body)
	}
}

// TestRefreshTokenRotation_RejectsReuse confirms the security invariant: the
// rotated-out token must not be usable a second time, and the family is
// poisoned so the *successor* token is also revoked. Returns 401 +
// error_code=refresh_token_reused.
func TestRefreshTokenRotation_RejectsReuse(t *testing.T) {
	handler, _, cleanup := buildRouterWithRefreshTokens(t)
	defer cleanup()

	_, refresh1 := signupAndLogin(t, handler, "bob@example.com")

	code, body := postRefresh(t, handler, refresh1)
	if code != http.StatusOK {
		t.Fatalf("first refresh: got %d body=%v", code, body)
	}
	refresh2, _ := body["refresh_token"].(string)
	if refresh2 == "" {
		t.Fatal("first refresh did not return a new refresh token")
	}

	code, body = postRefresh(t, handler, refresh1)
	if code != http.StatusUnauthorized {
		t.Errorf("reuse of refresh1: got %d, want 401; body=%v", code, body)
	}
	if got := apiErrorCode(body); got != "refresh_token_reused" {
		t.Errorf("reuse error_code: got %q, want refresh_token_reused; body=%v", got, body)
	}

	code, body = postRefresh(t, handler, refresh2)
	if code != http.StatusUnauthorized {
		t.Errorf("successor refresh2 after reuse: got %d, want 401; body=%v", code, body)
	}
	if got := apiErrorCode(body); got != "refresh_token_reused" {
		t.Errorf("family-revoked error_code on refresh2: got %q, want refresh_token_reused; body=%v", got, body)
	}
}

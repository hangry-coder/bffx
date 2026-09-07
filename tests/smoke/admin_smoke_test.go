package smoke

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
	"context"
)

// noopFlags satisfies featureflags.FlagProvider for admin router wiring in tests.
type noopFlags struct{}

func (noopFlags) Init(_ featureflags.ProviderConfig) error { return nil }
func (noopFlags) Close() error                         { return nil }
func (noopFlags) CreateFlag(_ featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, nil
}
func (noopFlags) UpdateFlag(_ string, _ featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, nil
}
func (noopFlags) DeleteFlag(_ string) error                          { return nil }
func (noopFlags) GetFlag(_ string) (*featureflags.FlagDefinition, error) { return nil, nil }
func (noopFlags) ListFlags() ([]featureflags.FlagDefinition, error)      { return nil, nil }
func (noopFlags) Evaluate(_ string, _ featureflags.EvalContext) (interface{}, error) {
	return nil, nil
}
func (noopFlags) EvaluateAll(_ featureflags.EvalContext) (map[string]featureflags.ResolvedFlag, error) {
	return map[string]featureflags.ResolvedFlag{}, nil
}
func (noopFlags) Subscribe(_ context.Context, _ func(string, interface{})) error { return nil }
func (noopFlags) Name() string                                                   { return "noop" }

func adminTestRouter(t *testing.T, store storage.Store) http.Handler {
	t.Helper()
	t.Setenv("BFFX_ADMIN_SESSION_KEY", "test-admin-session-secret-not-for-prod")
	t.Setenv("BFFX_ENV", "development")

	reg := &manifest.Registry{}
	jobs := worker.NewMemoryJobStore()
	bus := events.NewMemoryBus()
	aud := audit.NewAuditor(store)
	return admin.NewRouter(store, store, jobs, nil, reg, bus, aud, noopFlags{}, nil)
}

func TestAdminSmoke_LoginAndListResources(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := context.Background()
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, "AdminUser", map[string]any{
		"id":       "adm1",
		"email":    "admin@example.com",
		"password": hash,
		"role":     "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := adminTestRouter(t, store)

	// Login
	body := bytes.NewBufferString(`{"email":"admin@example.com","password":"secret123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login: status %d body %s", rr.Code, rr.Body.String())
	}
	var loginResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil || loginResp.Status != "ok" {
		t.Fatalf("login body: %v / %q", err, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == admin.SessionCookieName {
			session = c
			break
		}
	}
	if session == nil {
		t.Fatal("expected session cookie")
	}

	// List resources (protected)
	req2 := httptest.NewRequest(http.MethodGet, "/api/admin/resources", nil)
	req2.AddCookie(session)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("list resources: status %d body %s", rr2.Code, rr2.Body.String())
	}
}

func TestAdminSmoke_ProtectedRouteWithoutCookie(t *testing.T) {
	store := storage.NewMemoryStore()
	h := adminTestRouter(t, store)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/resources", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without session, got %d", rr.Code)
	}
}

func TestAdminSmoke_LoginInvalidPassword(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := context.Background()
	hash, _ := auth.HashPassword("right")
	_, _ = store.Create(ctx, "AdminUser", map[string]any{
		"id": "a", "email": "x@y.z", "password": hash, "role": "admin",
	})
	h := adminTestRouter(t, store)
	body := bytes.NewBufferString(`{"email":"x@y.z","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 invalid password, got %d", rr.Code)
	}
}

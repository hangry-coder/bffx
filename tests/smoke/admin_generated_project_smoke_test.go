package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
)

func loginAdminAndGetSession(t *testing.T, h http.Handler, email, password string) *http.Cookie {
	t.Helper()
	body := bytes.NewBufferString(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: status %d body %s", rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == admin.SessionCookieName {
			return c
		}
	}
	t.Fatal("expected admin session cookie")
	return nil
}

func flattenResourceGroups(groups map[string][]map[string]any) map[string]bool {
	names := make(map[string]bool)
	for _, list := range groups {
		for _, r := range list {
			if t, ok := r["type"].(string); ok && t == "virtual" {
				continue
			}
			if n, ok := r["name"].(string); ok && n != "" {
				names[n] = true
			}
		}
	}
	return names
}

func TestGeneratedProject_AdminPanel_FullRouteCoverage(t *testing.T) {
	t.Setenv("BFFX_ADMIN_SESSION_KEY", "test-admin-session-secret-not-for-prod")
	t.Setenv("BFFX_ENV", "development")

	tmpRoot := t.TempDir()
	const appName = "admin-generated"
	appRoot := filepath.Join(tmpRoot, appName)

	opts := generator.ProjectOptions{
		Layout:       generator.LayoutV2,
		StoreMode:    "memory",
		AdminEnabled: true,
		AdminEmail:   "admin@example.com",
		AdminPassword:"secret123",
		Minimal:      true,
	}
	if err := generator.ScaffoldNewProject(tmpRoot, appName, opts); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	if _, err := compiler.Sync(appRoot, false); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	reg, err := manifest.LoadAll(appRoot)
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}

	store := storage.NewMemoryStore()
	ctx := context.Background()
	hash, err := auth.HashPassword(opts.AdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, "AdminUser", map[string]any{
		"id":       "adm1",
		"email":    opts.AdminEmail,
		"password": hash,
		"role":     "superadmin",
	}); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	createdUser, err := store.Create(ctx, "User", map[string]any{
		"email":    "user@example.com",
		"password": "do-not-leak",
		"name":     "Normal User",
		"role":     "user",
		"status":   "active",
	})
	if err != nil {
		t.Fatalf("seed app user: %v", err)
	}

	h := admin.NewRouter(
		store,
		store,
		worker.NewMemoryJobStore(),
		nil,
		reg,
		events.NewMemoryBus(),
		audit.NewAuditor(store),
		noopFlags{},
		nil,
	)
	session := loginAdminAndGetSession(t, h, opts.AdminEmail, opts.AdminPassword)

	// 1) Every manifest resource that old admin exposed must still be visible in new admin resources API.
	reqRes := httptest.NewRequest(http.MethodGet, "/api/admin/resources", nil)
	reqRes.AddCookie(session)
	rrRes := httptest.NewRecorder()
	h.ServeHTTP(rrRes, reqRes)
	if rrRes.Code != http.StatusOK {
		t.Fatalf("resources endpoint: status %d body %s", rrRes.Code, rrRes.Body.String())
	}
	var grouped map[string][]map[string]any
	if err := json.Unmarshal(rrRes.Body.Bytes(), &grouped); err != nil {
		t.Fatalf("decode resources payload: %v", err)
	}
	seenNames := flattenResourceGroups(grouped)
	// Reserved admin resources (Feature Flag, Audit Log, Incident, AdminUser,
	// and any `bffx_*` internal table) intentionally do NOT show up under
	// "Other Resources" because they have dedicated admin surfaces. The
	// smoke test must mirror that contract so it does not regress the
	// dedup that fixes the duplicated `feature_flag` listing.
	reservedInAdmin := map[string]bool{
		"feature_flag": true, "FeatureFlag": true,
		"audit_log": true, "AuditLog": true,
		"incident":  true, "Incident": true,
		"AdminUser": true, "admin_user": true,
	}
	for _, m := range reg.Resources {
		name := m.Metadata.Name
		if reservedInAdmin[name] || strings.HasPrefix(name, "bffx_") {
			if seenNames[name] {
				t.Fatalf("reserved admin resource %q must NOT appear under Other Resources", name)
			}
			continue
		}
		if !seenNames[name] {
			t.Fatalf("resource %q missing from /api/admin/resources payload", name)
		}
	}

	// 2) Users directory works and strips sensitive fields.
	reqUsers := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	reqUsers.AddCookie(session)
	rrUsers := httptest.NewRecorder()
	h.ServeHTTP(rrUsers, reqUsers)
	if rrUsers.Code != http.StatusOK {
		t.Fatalf("users endpoint: status %d body %s", rrUsers.Code, rrUsers.Body.String())
	}
	var users []map[string]any
	if err := json.Unmarshal(rrUsers.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode users payload: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("expected at least one user in /api/admin/users")
	}
	for _, u := range users {
		if _, ok := u["password"]; ok {
			t.Fatal("users payload leaked password field")
		}
	}

	// 3) User detail endpoint returns enriched shape and strips sensitive fields.
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/admin/users/"+createdUser["id"].(string), nil)
	reqDetail.SetPathValue("id", createdUser["id"].(string))
	reqDetail.AddCookie(session)
	rrDetail := httptest.NewRecorder()
	h.ServeHTTP(rrDetail, reqDetail)
	if rrDetail.Code != http.StatusOK {
		t.Fatalf("user detail endpoint: status %d body %s", rrDetail.Code, rrDetail.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(rrDetail.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode user detail payload: %v", err)
	}
	userPart, ok := detail["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in detail payload: %v", detail)
	}
	if _, ok := userPart["password"]; ok {
		t.Fatal("user detail payload leaked password field")
	}

	// 4) Feature catalog endpoint is wired and returns the expected shape.
	reqFeat := httptest.NewRequest(http.MethodGet, "/api/admin/features", nil)
	reqFeat.AddCookie(session)
	rrFeat := httptest.NewRecorder()
	h.ServeHTTP(rrFeat, reqFeat)
	if rrFeat.Code != http.StatusOK {
		t.Fatalf("features endpoint: status %d body %s", rrFeat.Code, rrFeat.Body.String())
	}
	var feat struct {
		Groups          []map[string]any `json:"groups"`
		OrphanActions   []map[string]any `json:"orphan_actions"`
		FeatureClusters []map[string]any `json:"feature_clusters"`
	}
	if err := json.Unmarshal(rrFeat.Body.Bytes(), &feat); err != nil {
		t.Fatalf("decode features payload: %v", err)
	}
	// Generated minimal project has zero screens but the endpoint must still
	// respond with a well-formed envelope (not null) so the UI does not break.
	if feat.Groups == nil {
		t.Fatal("features payload groups must be a non-nil slice")
	}
	if feat.OrphanActions == nil {
		t.Fatal("features payload orphan_actions must be a non-nil slice")
	}
	if feat.FeatureClusters == nil {
		t.Fatal("features payload feature_clusters must be a non-nil slice")
	}
}

func TestGeneratedProject_AdminPanel_V2UIServesIndex(t *testing.T) {
	t.Setenv("BFFX_ADMIN_UI_V2", "true")
	t.Setenv("BFFX_ADMIN_SESSION_KEY", "test-admin-session-secret-not-for-prod")
	t.Setenv("BFFX_ENV", "development")

	reg := &manifest.Registry{}
	store := storage.NewMemoryStore()
	h := admin.NewRouter(
		store,
		store,
		worker.NewMemoryJobStore(),
		nil,
		reg,
		events.NewMemoryBus(),
		audit.NewAuditor(store),
		noopFlags{},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("v2 UI index status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := strings.ToLower(rr.Body.String())
	if !strings.Contains(body, "<!doctype html") && !strings.Contains(body, "<html") {
		t.Fatalf("v2 UI did not serve html index payload, got: %.80q", rr.Body.String())
	}
}


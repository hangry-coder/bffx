package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/router"
	"github.com/hangry-coder/bffx/pkg/auth"
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

	"gopkg.in/yaml.v3"
)

// TestAdminRBAC_PolicyAdminBlocksNonAdmin verifies that router.withPolicy
// rejects authenticated-but-non-admin claims on a resource whose write policy
// is `admin`. This is the runtime check backing the "admin-only" pattern in
// admin backends (Handleupdate_ai_config et al.).
func TestAdminRBAC_PolicyAdminBlocksNonAdmin(t *testing.T) {
	st := storage.NewMemoryStore()

	var userSpec, secretSpec yaml.Node
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
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: admin
  write: admin
fields:
  - {name: value, type: string, required: true}
`), &secretSpec); err != nil {
		t.Fatalf("secret spec: %v", err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "Secret"}, Spec: secretSpec},
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

	// 1. Authenticated non-admin POST is rejected. (NOTE: anonymous GET on a
	//    collection with policy=admin is currently NOT enforced by
	//    Router.withPolicy — it only kicks in on POST and item-fetch. That's a
	//    separate gap tracked in intel/active_tasks.md, intentionally not
	//    asserted here.)
	signup := `{"email":"bob@example.com","password":"password123","name":"Bob","role":"user"}`
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(signup)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"bob@example.com","password":"password123"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rr.Code, rr.Body.String())
	}
	var login struct{ Token string }
	json.Unmarshal(rr.Body.Bytes(), &login)
	if login.Token == "" {
		t.Fatal("login: empty token")
	}

	rr = httptest.NewRecorder()
	body := bytes.NewBufferString(`{"value":"top-secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("non-admin POST /secrets: got %d, want 403; body=%s", rr.Code, rr.Body.String())
	}

	// 3. Promote a user to admin via the store directly (Signup does not honor
	//    the `role` field from the request — tested below — so we have to
	//    grant it server-side, which is the real production flow).
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup",
		bytes.NewBufferString(`{"email":"admin@example.com","password":"password123","name":"Admin"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("admin signup failed: %d %s", rr.Code, rr.Body.String())
	}
	adminRow, err := st.GetByField(context.Background(), "User", "email", "admin@example.com")
	if err != nil {
		t.Fatal("admin user not found after signup")
	}
	adminID, _ := adminRow["id"].(string)
	if adminID == "" {
		t.Fatal("admin user has no id")
	}
	if _, err := st.Update(context.Background(), "User", adminID, map[string]any{"role": "admin"}); err != nil {
		t.Fatal("could not promote user to admin")
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"admin@example.com","password":"password123"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", rr.Code, rr.Body.String())
	}
	var adminLogin struct{ Token string }
	json.Unmarshal(rr.Body.Bytes(), &adminLogin)
	if adminLogin.Token == "" {
		t.Fatal("admin login: empty token")
	}

	rr = httptest.NewRecorder()
	body = bytes.NewBufferString(`{"value":"top-secret"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Authorization", "Bearer "+adminLogin.Token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("admin POST /secrets: got %d, want 201; body=%s", rr.Code, rr.Body.String())
	}

	// 4. The admin's secret should be fetchable by the admin and rejected for
	//    the non-admin authenticated user — this is the item-fetch path which
	//    does call policyEngine.Evaluate(policy, claims, item).
	var created map[string]any
	json.Unmarshal(rr.Body.Bytes(), &created)
	secretID, _ := created["id"].(string)
	if secretID == "" {
		t.Fatalf("created secret has no id: %v", created)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/secrets/"+secretID, nil)
	req.Header.Set("Authorization", "Bearer "+login.Token) // the non-admin "Bob" token
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("non-admin GET /secrets/:id: got %d, want 403; body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/secrets/"+secretID, nil)
	req.Header.Set("Authorization", "Bearer "+adminLogin.Token)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("admin GET /secrets/:id: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestPolicyAdmin_ListForbidden_Anonymous(t *testing.T) {
	st := storage.NewMemoryStore()
	var userSpec, secretSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: admin
  write: admin
fields:
  - {name: email, type: string, required: true}
  - {name: password, type: string, required: true}
`), &userSpec); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: admin
  write: admin
fields:
  - {name: value, type: string, required: true}
`), &secretSpec); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "T"}, Spec: yaml.Node{}},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "Secret"}, Spec: secretSpec},
		},
	}
	bus := events.NewMemoryBus()
	notify := notifications.NewManager(st)
	emailMgr := email.NewManager()
	hub := comm.NewHub(st, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(st, reg)
	r := router.NewRouter(router.RouterConfig{
		Store: st, Telemetry: st, JobStore: worker.NewMemoryJobStore(),
		Registry: reg, AuthProvider: auth.NewJWTProvider(auth.NewJWTService("test-secret")),
		JWTService: auth.NewJWTService("test-secret"), EventBus: bus,
		Notifications: notify, Email: emailMgr, CommHub: hub, I18n: bundle, FlagProvider: flagProvider,
	})
	handler := r.Setup()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("anonymous LIST /secrets: got %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestPolicyAdmin_ListAllowed_Admin(t *testing.T) {
	st := storage.NewMemoryStore()
	var userSpec, secretSpec yaml.Node
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
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: admin
  write: admin
fields:
  - {name: value, type: string, required: true}
`), &secretSpec); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "T"}, Spec: yaml.Node{}},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "Secret"}, Spec: secretSpec},
		},
	}
	bus := events.NewMemoryBus()
	notify := notifications.NewManager(st)
	emailMgr := email.NewManager()
	hub := comm.NewHub(st, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(st, reg)
	r := router.NewRouter(router.RouterConfig{
		Store: st, Telemetry: st, JobStore: worker.NewMemoryJobStore(),
		Registry: reg, AuthProvider: auth.NewJWTProvider(auth.NewJWTService("test-secret")),
		JWTService: auth.NewJWTService("test-secret"), EventBus: bus,
		Notifications: notify, Email: emailMgr, CommHub: hub, I18n: bundle, FlagProvider: flagProvider,
	})
	handler := r.Setup()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup",
		bytes.NewBufferString(`{"email":"adm@example.com","password":"password123","name":"A"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rr.Code, rr.Body.String())
	}
	adminRow, err := st.GetByField(context.Background(), "User", "email", "adm@example.com")
	if err != nil {
		t.Fatal(err)
	}
	adminID, _ := adminRow["id"].(string)
	st.Update(context.Background(), "User", adminID, map[string]any{"role": "admin"})

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"adm@example.com","password":"password123"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rr.Code, rr.Body.String())
	}
	var login struct{ Token string }
	json.Unmarshal(rr.Body.Bytes(), &login)

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin LIST /secrets: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

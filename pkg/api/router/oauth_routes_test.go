package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	"gopkg.in/yaml.v3"
)

func minimalRegistry(t *testing.T) *manifest.Registry {
	t.Helper()
	var userSpec yaml.Node
	_ = yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: email, type: string, required: true}
`), &userSpec)
	return &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project:   &manifest.Manifest{Metadata: manifest.Metadata{Name: "T"}, Spec: yaml.Node{}},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
		},
	}
}

func TestOAuthRoutes_DisabledByDefault_Returns404(t *testing.T) {
	t.Setenv("BFFX_ENABLE_OAUTH", "")
	s := storage.NewMemoryStore()
	reg := minimalRegistry(t)
	bus := events.NewMemoryBus()
	notify := notifications.NewManager(s)
	emailMgr := email.NewManager()
	hub := comm.NewHub(s, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	jwtS := auth.NewJWTService("s")
	fp := providers.NewBffxProvider(s, reg)
	r := NewRouter(RouterConfig{
		Store: s, Telemetry: s, JobStore: worker.NewMemoryJobStore(), Registry: reg,
		AuthProvider: jwtS, JWTService: jwtS, EventBus: bus, Notifications: notify, Email: emailMgr,
		CommHub: hub, I18n: bundle, FlagProvider: fp,
	})
	h := r.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when OAuth disabled, got %d", rr.Code)
	}
}

func TestOAuthRoutes_EnabledWhenEnvTrue(t *testing.T) {
	t.Setenv("BFFX_ENABLE_OAUTH", "true")
	s := storage.NewMemoryStore()
	reg := minimalRegistry(t)
	bus := events.NewMemoryBus()
	notify := notifications.NewManager(s)
	emailMgr := email.NewManager()
	hub := comm.NewHub(s, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	jwtS := auth.NewJWTService("s")
	fp := providers.NewBffxProvider(s, reg)
	r := NewRouter(RouterConfig{
		Store: s, Telemetry: s, JobStore: worker.NewMemoryJobStore(), Registry: reg,
		AuthProvider: jwtS, JWTService: jwtS, EventBus: bus, Notifications: notify, Email: emailMgr,
		CommHub: hub, I18n: bundle, FlagProvider: fp,
	})
	h := r.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when OAuth enabled, got %d", rr.Code)
	}
}

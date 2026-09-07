package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/api/router"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/worker"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/i18n"
)

func TestBootstrap(t *testing.T) {
	store := storage.NewMemoryStore()
	reg := manifest.NewRegistry("manifests")
	bus := events.NewLocalBus()
	notify := notifications.NewManager()
	emailMgr := email.NewManager()
	commHub := comm.NewHub()
	bundle := i18n.NewBundle()
	jobStore := worker.NewMemoryJobStore()

	jwtSvc := auth.NewJWTService("secret")
	r := router.NewRouter(router.RouterConfig{Store: store, Telemetry: store, JobStore: jobStore, Registry: reg, AuthProvider: auth.NewJWTProvider(jwtSvc), JWTService: jwtSvc, WorkerSecret: "worker-secret", EventBus: bus, Notifications: notify, Email: emailMgr, CommHub: commHub, I18n: bundle})
	handler := r.Setup()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/app/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
}
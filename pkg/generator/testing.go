package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateTests(projectDir, projectName string) error {
	testDir := filepath.Join(projectDir, "tests")
	e2eDir := filepath.Join(testDir, "e2e")
	unitDir := filepath.Join(testDir, "unit")

	if err := os.MkdirAll(e2eDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return err
	}

	// 1. tests/e2e/health_test.go
	healthTest := strings.Join([]string{
		"package e2e",
		"",
		"import (",
		"	\"net/http\"",
		"	\"net/http/httptest\"",
		"	\"testing\"",
		"	\"github.com/hangry-coder/bffx/pkg/auth\"",
		"	\"github.com/hangry-coder/bffx/pkg/manifest\"",
		"	\"github.com/hangry-coder/bffx/pkg/storage\"",
		"	\"github.com/hangry-coder/bffx/pkg/api/router\"",
		"	\"github.com/hangry-coder/bffx/pkg/events\"",
		"	\"github.com/hangry-coder/bffx/pkg/worker\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm/notifications\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm/email\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm\"",
		"	\"github.com/hangry-coder/bffx/pkg/i18n\"",
		")",
		"",
		"func TestHealth(t *testing.T) {",
		"	// Setup",
		"	store := storage.NewMemoryStore()",
		"	reg, err := manifest.LoadAll(\"../..\")",
		"	if err != nil {",
		"		t.Fatalf(\"load manifest: %v\", err)",
		"	}",
		"	bus := events.NewMemoryBus()",
		"	notify := notifications.NewManager(store)",
		"	emailMgr := email.NewManager()",
		"	emailMgr.RegisterProvider(\"default\", &email.LogProvider{})",
		"	tmplMgr := comm.NewTemplateManager(reg)",
		"	commHub := comm.NewHub(store, notify, emailMgr, tmplMgr)",
		"	bundle := i18n.NewBundle(\"en\")",
		"	jobStore := worker.NewMemoryJobStore()",
		"",
		"	jwtSvc := auth.NewJWTService(\"secret\")",
		"	r := router.NewRouter(router.RouterConfig{Store: store, Telemetry: store, JobStore: jobStore, Registry: reg, AuthProvider: auth.NewJWTProvider(jwtSvc), JWTService: jwtSvc, WorkerSecret: \"worker-secret\", EventBus: bus, Notifications: notify, Email: emailMgr, CommHub: commHub, I18n: bundle})",
		"	handler := r.Setup()",
		"",
		"	// Test",
		"	ts := httptest.NewServer(handler)",
		"	defer ts.Close()",
		"",
		"	res, err := http.Get(ts.URL + \"/health\")",
		"	if err != nil {",
		"		t.Fatal(err)",
		"	}",
		"	if res.StatusCode != http.StatusOK {",
		"		t.Errorf(\"expected status 200, got %d\", res.StatusCode)",
		"	}",
		"}",
	}, "\n")
	os.WriteFile(filepath.Join(e2eDir, "health_test.go"), []byte(healthTest), 0o644)

	// 2. tests/e2e/bootstrap_test.go
	bootstrapTest := strings.Join([]string{
		"package e2e",
		"",
		"import (",
		"	\"net/http\"",
		"	\"net/http/httptest\"",
		"	\"testing\"",
		"	\"github.com/hangry-coder/bffx/pkg/auth\"",
		"	\"github.com/hangry-coder/bffx/pkg/storage\"",
		"	\"github.com/hangry-coder/bffx/pkg/manifest\"",
		"	\"github.com/hangry-coder/bffx/pkg/api/router\"",
		"	\"github.com/hangry-coder/bffx/pkg/events\"",
		"	\"github.com/hangry-coder/bffx/pkg/worker\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm/notifications\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm/email\"",
		"	\"github.com/hangry-coder/bffx/pkg/comm\"",
		"	\"github.com/hangry-coder/bffx/pkg/i18n\"",
		")",
		"",
		"func TestBootstrap(t *testing.T) {",
		"	store := storage.NewMemoryStore()",
		"	reg, err := manifest.LoadAll(\"../..\")",
		"	if err != nil {",
		"		t.Fatalf(\"load manifest: %v\", err)",
		"	}",
		"	bus := events.NewMemoryBus()",
		"	notify := notifications.NewManager(store)",
		"	emailMgr := email.NewManager()",
		"	emailMgr.RegisterProvider(\"default\", &email.LogProvider{})",
		"	tmplMgr := comm.NewTemplateManager(reg)",
		"	commHub := comm.NewHub(store, notify, emailMgr, tmplMgr)",
		"	bundle := i18n.NewBundle(\"en\")",
		"	jobStore := worker.NewMemoryJobStore()",
		"",
		"	jwtSvc := auth.NewJWTService(\"secret\")",
		"	r := router.NewRouter(router.RouterConfig{Store: store, Telemetry: store, JobStore: jobStore, Registry: reg, AuthProvider: auth.NewJWTProvider(jwtSvc), JWTService: jwtSvc, WorkerSecret: \"worker-secret\", EventBus: bus, Notifications: notify, Email: emailMgr, CommHub: commHub, I18n: bundle})",
		"	handler := r.Setup()",
		"",
		"	ts := httptest.NewServer(handler)",
		"	defer ts.Close()",
		"",
		"	req, err := http.NewRequest(\"GET\", ts.URL + \"/api/v1/app/bootstrap\", nil)",
		"	if err != nil {",
		"		t.Fatal(err)",
		"	}",
		"	req.Header.Set(\"X-BFFX-Client-Version\", \"1.0.0\")",
		"	res, err := http.DefaultClient.Do(req)",
		"	if err != nil {",
		"		t.Fatal(err)",
		"	}",
		"	if res.StatusCode != http.StatusOK {",
		"		t.Errorf(\"expected status 200, got %d\", res.StatusCode)",
		"	}",
		"}",
	}, "\n")
	os.WriteFile(filepath.Join(e2eDir, "bootstrap_test.go"), []byte(bootstrapTest), 0o644)

	// 3. tests/unit/hooks_test.go
	hooksTest := strings.Join([]string{
		"package unit",
		"",
		"import (",
		"	\"testing\"",
		"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
		"	\"github.com/hangry-coder/bffx/pkg/storage\"",
		")",
		"",
		"func TestExampleHook(t *testing.T) {",
		"	store := storage.NewMemoryStore()",
		"	ctx := &handlers.ActionContext{",
		"		Store: store,",
		"	}",
		"	_ = ctx",
		"",
		"	// Mock payload",
		"	payload := map[string]any{",
		"		\"name\": \"Test\",",
		"	}",
		"",
		"	// In a real app, you would import your hooks and call them here",
		"	// err := hooks.MyHook(ctx, payload)",
		"	// if err != nil { t.Error(err) }",
		"	",
		"	if payload[\"name\"] != \"Test\" {",
		"		t.Errorf(\"expected Test, got %v\", payload[\"name\"])",
		"	}",
		"}",
	}, "\n")
	os.WriteFile(filepath.Join(unitDir, "hooks_test.go"), []byte(hooksTest), 0o644)

	fmt.Printf("Scaffolded tests in %s\n", testDir)
	return nil
}

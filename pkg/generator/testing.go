package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/version"
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
		fmt.Sprintf("\t\"%s/pkg/auth\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/manifest\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/storage\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/api/router\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/events\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/worker\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm/notifications\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm/email\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/i18n\"", version.FrameworkModulePath()),
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
		fmt.Sprintf("\t\"%s/pkg/auth\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/storage\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/manifest\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/api/router\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/events\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/worker\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm/notifications\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm/email\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/comm\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/i18n\"", version.FrameworkModulePath()),
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
		fmt.Sprintf("\t\"%s/pkg/api/handlers\"", version.FrameworkModulePath()),
		fmt.Sprintf("\t\"%s/pkg/storage\"", version.FrameworkModulePath()),
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

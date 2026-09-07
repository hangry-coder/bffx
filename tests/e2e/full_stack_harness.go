package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

const (
	fullStackAppPrefix   = "e2e-full-stack"
	fullStackAdminEmail  = "e2e-admin@example.com"
	fullStackAdminPass   = "e2e-admin-password-32chars!!"
	fullStackUserEmail   = "e2e-user@example.com"
	fullStackUserPass    = "e2e-user-password-32chars!!!"
	fullStackWirePackage = "e2efull.wire.v1"
)

type fullStackEnv struct {
	t          *testing.T
	reporter   *TestReporter
	bffxBin    string
	moduleRoot string
	projectDir string
	appName    string
	storeMode  string
	httpPort   int
	grpcPort   int
	logPath    string
	logWriter  io.Writer
	serverCmd  *exec.Cmd
}

func runFullStackHarness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-stack e2e in short mode")
	}

	httpPort := mustFreePort(t)
	grpcPort := mustFreePort(t)

	wd, _ := os.Getwd()
	logsDir := filepath.Join(wd, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	timestamp := time.Now().Format("20060102_150405")
	appName := fmt.Sprintf("%s-%s", fullStackAppPrefix, timestamp)
	logPath := filepath.Join(logsDir, fmt.Sprintf("%s_%s_server.log", fullStackAppPrefix, timestamp))
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create server log: %v", err)
	}
	defer logFile.Close()

	storeMode := resolveE2EStoreMode()
	env := &fullStackEnv{
		t:          t,
		reporter:   NewTestReporter("TestFullStackIntegration", appName, storeMode, httpPort, grpcPort, logPath),
		appName:    appName,
		storeMode:  storeMode,
		httpPort:   httpPort,
		grpcPort:   grpcPort,
		logPath:    logPath,
		logWriter:  io.MultiWriter(logFile, os.Stdout),
	}

	defer func() {
		reportPath, err := env.reporter.Finish(logsDir)
		if err != nil {
			t.Logf("failed to write report: %v", err)
		} else {
			t.Logf("full-stack report: %s", reportPath)
		}
	}()

	defer env.cleanupProject()
	defer env.stopServer()

	moduleRoot, err := bffxModuleRoot()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	env.moduleRoot = moduleRoot

	bffxBin, err := bffxCLIPath()
	if err != nil {
		t.Fatalf("bffx cli: %v", err)
	}
	env.bffxBin = bffxBin

	testProjectsDir := filepath.Join(moduleRoot, "test-projects")
	_ = os.MkdirAll(testProjectsDir, 0o755)
	env.projectDir = filepath.Join(testProjectsDir, appName)

	env.scaffold(testProjectsDir)
	env.generateArtifacts()
	env.enableWire()
	env.syncProject()
	env.assertMainWiring()
	env.buildDevBinary()
	env.startServer()
	env.waitHealthy()

	token := env.testAuthAndCRUD()
	env.testCustomActionREST(token)
	env.testAdminPanel()
	env.testGRPC(token)
	env.testDBPersistence(token)

	if env.reporter.HasFailures() {
		t.Fatalf("full-stack e2e had failures — see report under tests/e2e/logs and server log %s", logPath)
	}
}

func resolveE2EStoreMode() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("E2E_STORE"))); v != "" {
		return v
	}
	if url := strings.TrimSpace(os.Getenv("E2E_DATABASE_URL")); url != "" {
		return "postgres"
	}
	if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" && os.Getenv("E2E_USE_DATABASE_URL") == "true" {
		return "postgres"
	}
	return "sqlite"
}

func mustFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func (e *fullStackEnv) runStep(name string, fn func() (detail string, err error)) {
	start := time.Now()
	detail, err := fn()
	if err != nil {
		e.reporter.Fail(name, fmt.Sprintf("%s: %v", detail, err), time.Since(start))
		e.t.Logf("FAIL %s: %v (%s)", name, err, detail)
		return
	}
	e.reporter.Pass(name, detail, time.Since(start))
	e.t.Logf("PASS %s: %s", name, detail)
}

func (e *fullStackEnv) scaffold(parentDir string) {
	e.runStep("scaffold_app", func() (string, error) {
		_ = os.RemoveAll(e.projectDir)
		args := []string{
			"new", e.appName,
			"--non-interactive", "--dev-vendor",
			"--admin-email", fullStackAdminEmail,
			"--admin-password", fullStackAdminPass,
			"--store", e.storeMode,
		}
		cmd := exec.Command(e.bffxBin, args...)
		cmd.Dir = parentDir
		cmd.Stdout = e.logWriter
		cmd.Stderr = e.logWriter
		if err := cmd.Run(); err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(e.projectDir, "cmd", "api", "main.go")); err != nil {
			return "", fmt.Errorf("cmd/api/main.go missing: %w", err)
		}
		stripAppSecretFromEnv(e.projectDir)
		if e.storeMode == "postgres" {
			if err := patchPostgresURL(e.projectDir); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("project at %s (store=%s)", e.projectDir, e.storeMode), nil
	})
}

func stripAppSecretFromEnv(projectDir string) {
	envPath := filepath.Join(projectDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "BFFX_APP_SECRET=") {
			continue
		}
		lines = append(lines, line)
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o644)
}

func patchPostgresURL(projectDir string) error {
	url := strings.TrimSpace(os.Getenv("E2E_DATABASE_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if url == "" {
		return fmt.Errorf("postgres store requested but E2E_DATABASE_URL/DATABASE_URL unset")
	}
	envPath := filepath.Join(projectDir, ".env")
	data, _ := os.ReadFile(envPath)
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "DATABASE_URL=") {
			lines[i] = "DATABASE_URL=" + url
			found = true
		}
	}
	if !found {
		lines = append(lines, "DATABASE_URL="+url)
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o644)
}

func (e *fullStackEnv) generateArtifacts() {
	e.runStep("generate_resource_note", func() (string, error) {
		cmd := exec.Command(e.bffxBin, "generate", "resource", "Note", "content:string")
		cmd.Dir = e.projectDir
		cmd.Stdout = e.logWriter
		cmd.Stderr = e.logWriter
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return "resource Note (content:string)", nil
	})

	e.runStep("generate_action_ping", func() (string, error) {
		cmd := exec.Command(e.bffxBin, "generate", "action", "Ping")
		cmd.Dir = e.projectDir
		cmd.Stdout = e.logWriter
		cmd.Stderr = e.logWriter
		if err := cmd.Run(); err != nil {
			return "", err
		}
		hookPath := filepath.Join(e.projectDir, "internal", "features", "app", "hooks", "ping.go")
		hook := `package hooks

import (
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

func HandlePing(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	errors.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "via": "rest"})
}
`
		if err := os.WriteFile(hookPath, []byte(hook), 0o644); err != nil {
			return "", err
		}
		return "action Ping with HandlePing hook", nil
	})
}

func (e *fullStackEnv) enableWire() {
	e.runStep("enable_wire", func() (string, error) {
		yamlPath := filepath.Join(e.projectDir, "bffx", "project.yaml")
		content, err := os.ReadFile(yamlPath)
		if err != nil {
			return "", err
		}
		text := string(content)
		wireBlock := fmt.Sprintf(
			"    wire:\n      enabled: true\n      grpc_port: %d\n      package: %q\n",
			e.grpcPort, fullStackWirePackage,
		)
		switch {
		case strings.Contains(text, "    streaming:\n      enabled: false"):
			text = strings.Replace(text, "    streaming:\n      enabled: false", "    streaming:\n      enabled: false\n"+wireBlock, 1)
		case strings.Contains(text, "    streaming:\n      enabled: true"):
			text = strings.Replace(text, "    streaming:\n      enabled: true", "    streaming:\n      enabled: true\n"+wireBlock, 1)
		default:
			return "", fmt.Errorf("could not find streaming block in project.yaml")
		}
		if err := os.WriteFile(yamlPath, []byte(text), 0o644); err != nil {
			return "", err
		}
		manifest.InvalidateLoadAllCache(e.projectDir)
		return fmt.Sprintf("wire enabled on grpc_port=%d package=%s", e.grpcPort, fullStackWirePackage), nil
	})
}

func (e *fullStackEnv) syncProject() {
	e.runStep("bffx_sync", func() (string, error) {
		cmd := exec.Command(e.bffxBin, "sync")
		cmd.Dir = e.projectDir
		cmd.Stdout = e.logWriter
		cmd.Stderr = e.logWriter
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return "sync completed", nil
	})
}

func (e *fullStackEnv) assertMainWiring() {
	e.runStep("main_go_wires_handlers", func() (string, error) {
		mainGo, err := os.ReadFile(filepath.Join(e.projectDir, "cmd", "api", "main.go"))
		if err != nil {
			return "", err
		}
		text := string(mainGo)
		if !strings.Contains(text, "ActionHandlers, HookHandlers") {
			return "", fmt.Errorf("main.go missing ActionHandlers/HookHandlers wiring")
		}
		return "cmd/api/main.go passes ActionHandlers and HookHandlers", nil
	})
}

func (e *fullStackEnv) buildDevBinary() {
	e.runStep("dev_build_cmd_api", func() (string, error) {
		out := filepath.Join(e.projectDir, ".bffx", "orchestrator")
		_ = os.MkdirAll(filepath.Dir(out), 0o755)
		cmd := exec.Command("go", "build", "-o", out, "./cmd/api")
		cmd.Dir = e.projectDir
		cmd.Env = compiler.GoBuildEnv()
		cmd.Stdout = e.logWriter
		cmd.Stderr = e.logWriter
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return "built .bffx/orchestrator from ./cmd/api (bffx dev path)", nil
	})
}

func (e *fullStackEnv) serverEnv() []string {
	block := []string{
		"BFFX_JWT_SECRET=", "BFFX_ADMIN_SESSION_KEY=", "BFFX_APP_SECRET=", "BFFX_WORKER_SECRET=",
	}
	var out []string
loop:
	for _, item := range os.Environ() {
		for _, p := range block {
			if strings.HasPrefix(item, p) {
				continue loop
			}
		}
		out = append(out, item)
	}
	out = append(out,
		fmt.Sprintf("PORT=%d", e.httpPort),
		"BFFX_JWT_SECRET=e2e-full-stack-jwt-secret-at-least-32-chars!!!",
		"BFFX_ADMIN_SESSION_KEY=e2e-full-stack-admin-session-key-32chars!!!",
		"BFFX_HTTP_ACCESS_LOG=off",
	)
	if e.storeMode == "postgres" {
		if url := strings.TrimSpace(os.Getenv("E2E_DATABASE_URL")); url != "" {
			out = append(out, "DATABASE_URL="+url)
		} else if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" {
			out = append(out, "DATABASE_URL="+url)
		}
	}
	return out
}

func (e *fullStackEnv) startServer() {
	e.runStep("start_dev_server", func() (string, error) {
		bin := filepath.Join(e.projectDir, ".bffx", "orchestrator")
		e.serverCmd = exec.Command(bin)
		e.serverCmd.Dir = e.projectDir
		e.serverCmd.Env = e.serverEnv()
		e.serverCmd.Stdout = e.logWriter
		e.serverCmd.Stderr = e.logWriter
		if err := e.serverCmd.Start(); err != nil {
			return "", err
		}
		return fmt.Sprintf("pid=%d http=:%d grpc=:%d", e.serverCmd.Process.Pid, e.httpPort, e.grpcPort), nil
	})
}

func (e *fullStackEnv) stopServer() {
	if e.serverCmd != nil && e.serverCmd.Process != nil {
		_ = e.serverCmd.Process.Kill()
		_, _ = e.serverCmd.Process.Wait()
		e.serverCmd = nil
	}
}

func (e *fullStackEnv) waitHealthy() {
	e.runStep("health_check", func() (string, error) {
		url := fmt.Sprintf("http://127.0.0.1:%d/health", e.httpPort)
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return url + " -> 200", nil
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(500 * time.Millisecond)
		}
		return "", fmt.Errorf("timeout waiting for %s", url)
	})
}

func (e *fullStackEnv) baseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", e.httpPort)
}

func (e *fullStackEnv) testAuthAndCRUD() string {
	var token string
	e.runStep("rest_signup_login", func() (string, error) {
		signupURL := e.baseURL() + "/api/v1/auth/signup"
		payload := map[string]string{
			"email": fullStackUserEmail, "password": fullStackUserPass, "name": "E2E User",
		}
		body, _ := json.Marshal(payload)
		resp, err := http.Post(signupURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("signup status=%d body=%s", resp.StatusCode, string(b))
		}

		loginURL := e.baseURL() + "/api/v1/auth/login"
		loginBody, _ := json.Marshal(map[string]string{"email": fullStackUserEmail, "password": fullStackUserPass})
		resp, err = http.Post(loginURL, "application/json", bytes.NewReader(loginBody))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("login status=%d body=%s", resp.StatusCode, string(b))
		}
		var loginResult struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&loginResult); err != nil {
			return "", err
		}
		if loginResult.Token == "" {
			return "", fmt.Errorf("login returned empty token")
		}
		token = loginResult.Token
		return "signup + login OK", nil
	})

	var noteID string
	e.runStep("rest_create_note", func() (string, error) {
		if token == "" {
			return "", fmt.Errorf("missing auth token from prior step")
		}
		noteBody, _ := json.Marshal(map[string]string{"content": "hello from e2e"})
		req, _ := http.NewRequest(http.MethodPost, e.baseURL()+"/api/v1/notes", bytes.NewReader(noteBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("create note status=%d body=%s", resp.StatusCode, string(b))
		}
		var note map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
			return "", err
		}
		id, _ := note["id"].(string)
		if id == "" {
			return "", fmt.Errorf("create note missing id: %v", note)
		}
		noteID = id
		return "created note id=" + id, nil
	})

	e.runStep("rest_read_note", func() (string, error) {
		if token == "" || noteID == "" {
			return "", fmt.Errorf("missing token or note id")
		}
		req, _ := http.NewRequest(http.MethodGet, e.baseURL()+"/api/v1/notes/"+noteID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("get note status=%d body=%s", resp.StatusCode, string(b))
		}
		var note map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
			return "", err
		}
		content, _ := note["content"].(string)
		if content != "hello from e2e" {
			return "", fmt.Errorf("unexpected content %q", content)
		}
		return "read note content OK", nil
	})

	return token
}

func (e *fullStackEnv) testCustomActionREST(token string) {
	e.runStep("rest_custom_action_ping", func() (string, error) {
		if token == "" {
			return "", fmt.Errorf("missing auth token")
		}
		req, _ := http.NewRequest(http.MethodPost, e.baseURL()+"/api/v1/actions/ping", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("action route 404 — main.go likely not wiring ActionHandlers")
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("ping status=%d body=%s", resp.StatusCode, string(b))
		}
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}
		if result["ok"] != true {
			return "", fmt.Errorf("unexpected body %v", result)
		}
		return "POST /api/v1/actions/ping -> 200 ok=true", nil
	})
}

func (e *fullStackEnv) testAdminPanel() {
	e.runStep("admin_spa_health", func() (string, error) {
		resp, err := http.Get(e.baseURL() + "/admin/")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("admin SPA status=%d", resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(b), "<") {
			return "", fmt.Errorf("admin SPA response does not look like HTML")
		}
		return "/admin/ -> 200 HTML", nil
	})

	var sessionCookie *http.Cookie
	e.runStep("admin_login_session", func() (string, error) {
		body, _ := json.Marshal(map[string]string{"email": fullStackAdminEmail, "password": fullStackAdminPass})
		resp, err := http.Post(e.baseURL()+"/admin/api/admin/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("admin login status=%d body=%s", resp.StatusCode, string(b))
		}
		for _, c := range resp.Cookies() {
			if c.Name == "bffx_admin_session" {
				sessionCookie = c
				break
			}
		}
		if sessionCookie == nil {
			return "", fmt.Errorf("admin session cookie missing")
		}
		return "admin login + session cookie", nil
	})

	e.runStep("admin_session_api", func() (string, error) {
		if sessionCookie == nil {
			return "", fmt.Errorf("missing admin session")
		}
		req, _ := http.NewRequest(http.MethodGet, e.baseURL()+"/admin/api/admin/session", nil)
		req.AddCookie(sessionCookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("admin session status=%d body=%s", resp.StatusCode, string(b))
		}
		return "GET /admin/api/admin/session -> 200", nil
	})

	e.runStep("admin_resources_api", func() (string, error) {
		if sessionCookie == nil {
			return "", fmt.Errorf("missing admin session")
		}
		req, _ := http.NewRequest(http.MethodGet, e.baseURL()+"/admin/api/admin/resources", nil)
		req.AddCookie(sessionCookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("admin resources status=%d body=%s", resp.StatusCode, string(b))
		}
		var grouped map[string][]map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&grouped); err != nil {
			return "", err
		}
		foundNote := false
		for _, list := range grouped {
			for _, r := range list {
				if n, _ := r["name"].(string); strings.EqualFold(n, "Note") {
					foundNote = true
				}
			}
		}
		if !foundNote {
			return "", fmt.Errorf("Note resource missing from admin resources payload")
		}
		return "GET /admin/api/admin/resources lists Note", nil
	})
}

func (e *fullStackEnv) testGRPC(token string) {
	grpcStub := filepath.Join(e.projectDir, ".bffx", "gen", "go", "proto", "bffx", "v1", "service_grpc.pb.go")
	if _, err := os.Stat(grpcStub); err != nil {
		e.reporter.Skip("grpc_action_ping", "buf generate stubs missing — install buf and protoc-gen-go-grpc to enable gRPC assertions")
		e.reporter.Skip("grpc_create_note", "skipped (no gRPC stubs)")
		return
	}

	if err := e.writeGRPCProbe(); err != nil {
		e.reporter.Fail("grpc_probe_setup", err.Error(), 0)
		return
	}

	e.runStep("grpc_action_ping", func() (string, error) {
		return e.runGRPCProbe("ping", token)
	})
	e.runStep("grpc_create_note", func() (string, error) {
		return e.runGRPCProbe("create-note", token)
	})
}

func (e *fullStackEnv) writeGRPCProbe() error {
	modPath := readModulePath(e.projectDir)
	if modPath == "" {
		return fmt.Errorf("could not read go.mod module path")
	}
	probeDir := filepath.Join(e.projectDir, "cmd", "e2e-grpc-probe")
	_ = os.MkdirAll(probeDir, 0o755)
	src := fmt.Sprintf(`package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	pb "%s/.bffx/gen/go/proto/bffx/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: e2e-grpc-probe <ping|create-note>")
		os.Exit(2)
	}
	host := os.Getenv("GRPC_HOST")
	if host == "" {
		host = "127.0.0.1:%d"
	}
	token := os.Getenv("JWT_TOKEN")
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fail(err)
	}
	defer conn.Close()

	ctx := context.Background()
	if token != "" {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
	}

	client := pb.NewActionServiceClient(conn)
	noteClient := pb.NewNoteServiceClient(conn)

	switch os.Args[1] {
	case "ping":
		resp, err := client.Ping(ctx, &pb.PingRequest{})
		if err != nil {
			fail(err)
		}
		if resp.GetResult() == nil || resp.GetResult().GetFields()["ok"].GetBoolValue() != true {
			fail(fmt.Errorf("unexpected ping result: %%v", resp.GetResult()))
		}
		ok(map[string]any{"via": "grpc", "action": "Ping"})
	case "create-note":
		resp, err := noteClient.CreateNote(ctx, &pb.CreateNoteRequest{Content: "hello from grpc"})
		if err != nil {
			fail(err)
		}
		if resp.GetId() == "" {
			fail(fmt.Errorf("create note missing id"))
		}
		ok(map[string]any{"id": resp.GetId(), "via": "grpc"})
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand")
		os.Exit(2)
	}
}

func ok(v map[string]any) {
	_ = json.NewEncoder(os.Stdout).Encode(v)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
`, modPath, e.grpcPort)
	return os.WriteFile(filepath.Join(probeDir, "main.go"), []byte(src), 0o644)
}

func (e *fullStackEnv) runGRPCProbe(subcmd, token string) (string, error) {
	probeBin := filepath.Join(e.projectDir, ".bffx", "grpc-probe")
	build := exec.Command("go", "build", "-o", probeBin, "./cmd/e2e-grpc-probe")
	build.Dir = e.projectDir
	build.Env = append(compiler.GoBuildEnv(), "GOWORK=off")
	build.Stdout = e.logWriter
	build.Stderr = e.logWriter
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("build grpc probe: %w", err)
	}

	cmd := exec.Command(probeBin, subcmd)
	cmd.Dir = e.projectDir
	cmd.Env = append(e.serverEnv(),
		"GOWORK=off",
		fmt.Sprintf("GRPC_HOST=127.0.0.1:%d", e.grpcPort),
		"JWT_TOKEN="+token,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w\n%s", err, string(out))
	}
	var result map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		return "", fmt.Errorf("decode probe output: %v raw=%s", err, string(out))
	}
	return fmt.Sprintf("%s -> %v", subcmd, result), nil
}

func (e *fullStackEnv) testDBPersistence(token string) {
	var noteID string

	e.runStep("db_persist_create_note", func() (string, error) {
		if token == "" {
			return "", fmt.Errorf("missing auth token")
		}
		noteBody, _ := json.Marshal(map[string]string{"content": "persist me"})
		req, _ := http.NewRequest(http.MethodPost, e.baseURL()+"/api/v1/notes", bytes.NewReader(noteBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
		}
		var note map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&note)
		noteID, _ = note["id"].(string)
		if noteID == "" {
			return "", fmt.Errorf("missing note id")
		}
		return "created persist note id=" + noteID, nil
	})

	e.stopServer()
	time.Sleep(500 * time.Millisecond)
	e.startServer()
	e.waitHealthy()

	e.runStep("db_persist_read_after_restart", func() (string, error) {
		if token == "" || noteID == "" {
			return "", fmt.Errorf("missing token or note id")
		}
		req, _ := http.NewRequest(http.MethodGet, e.baseURL()+"/api/v1/notes/"+noteID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
		}
		var note map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
			return "", err
		}
		content, _ := note["content"].(string)
		if content != "persist me" {
			return "", fmt.Errorf("expected %q got %q", "persist me", content)
		}
		return fmt.Sprintf("note %s survived server restart (store=%s)", noteID, e.storeMode), nil
	})
}

func (e *fullStackEnv) cleanupProject() {
	e.runStep("cleanup_test_app", func() (string, error) {
		if e.projectDir == "" {
			return "no project dir", nil
		}
		if err := os.RemoveAll(e.projectDir); err != nil {
			return "", err
		}
		return "removed " + e.projectDir, nil
	})
}

func readModulePath(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

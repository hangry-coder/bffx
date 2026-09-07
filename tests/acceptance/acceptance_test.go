// Package acceptance runs an optional full-stack smoke (temp scaffold + bffx dev + /health).
//
// Canonical integration coverage lives under tests/e2e (test-cicd, test-deploy stacks).
// Do not duplicate brittle CRUD/API assertions here—policy, secrets, and headers evolve independently.
package acceptance

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// acceptanceServerEnv returns env for spawned processes without inherited weak BFFX secrets
// (ValidateEnv rejects short JWT/session keys even in development).
func acceptanceServerEnv(port int) []string {
	block := []string{
		"BFFX_JWT_SECRET=", "BFFX_ADMIN_SESSION_KEY=", "BFFX_APP_SECRET=", "BFFX_WORKER_SECRET=",
	}
	var out []string
loop:
	for _, e := range os.Environ() {
		for _, p := range block {
			if strings.HasPrefix(e, p) {
				continue loop
			}
		}
		out = append(out, e)
	}
	out = append(out,
		fmt.Sprintf("PORT=%d", port),
		"BFFX_JWT_SECRET=acceptance-test-jwt-secret-at-least-32-chars!!!",
		"BFFX_ADMIN_SESSION_KEY=acceptance-admin-session-key-at-least-32-chars!!!",
	)
	return out
}

func TestFullScaffoldFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping acceptance test in short mode")
	}

	root, err := os.MkdirTemp("", "bffx-acceptance-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// Repo root: tests/acceptance -> ../..
	workspaceRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))

	// 1. Build bffx binary
	binPath := filepath.Join(root, "bffx")
	buildCmd := exec.Command("go", "build", "-o", binPath, filepath.Join(workspaceRoot, "cmd/bffx"))
	buildCmd.Dir = workspaceRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build bffx: %v\n%s", err, out)
	}

	// 2. bffx new — run from workspace so --dev-vendor can find pkg/; --root places the app under tmp
	appName := "testapp"
	newCmd := exec.Command(binPath, "new", appName, "--root", root, "--non-interactive", "--dev-vendor")
	newCmd.Dir = workspaceRoot
	if out, err := newCmd.CombinedOutput(); err != nil {
		t.Fatalf("bffx new failed: %v\n%s", err, out)
	}

	projectDir := filepath.Join(root, appName)

	// 3. bffx generate resource (exercises generate → sync pipeline in isolation from release semantics)
	genCmd := exec.Command(binPath, "generate", "resource", "Note", "content:string")
	genCmd.Dir = projectDir
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("bffx generate failed: %v\n%s", err, out)
	}

	// 4. bffx sync
	syncCmd := exec.Command(binPath, "sync")
	syncCmd.Dir = projectDir
	if out, err := syncCmd.CombinedOutput(); err != nil {
		t.Fatalf("bffx sync failed: %v\n%s", err, out)
	}

	// 5. Start dev server
	// We run it in the background and wait for it to be ready
	port := 9091
	serverCmd := exec.Command(binPath, "dev")
	serverCmd.Dir = projectDir
	serverCmd.Env = acceptanceServerEnv(port)

	var serverOutput strings.Builder
	serverCmd.Stdout = &serverOutput
	serverCmd.Stderr = &serverOutput

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("failed to start dev server: %v", err)
	}
	defer serverCmd.Process.Kill()

	// Wait for server to start (polling health; prefer IPv4 loopback)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	maxRetries := 40
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err = http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if err != nil || resp == nil || resp.StatusCode != 200 {
		t.Errorf("server health check failed: %v", err)
		t.Fatalf("Server Output:\n%s", serverOutput.String())
	}
	resp.Body.Close()

	t.Log("acceptance smoke passed (health); see tests/e2e for CRUD and deployment stacks")
}

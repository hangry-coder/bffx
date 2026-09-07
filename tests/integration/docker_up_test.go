//go:build integration

package integration

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/deploy"
)

func checkDocker(t *testing.T) {
	t.Helper()
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Skipping Docker integration test: Docker daemon is not running")
	}
}

// TestBffxUpStackStartsHealthy runs `docker compose up -d` on a scaffolded
// project and waits for the /health endpoint to return 200.
func TestBffxUpStackStartsHealthy(t *testing.T) {
	checkDocker(t)
	root := t.TempDir()

	// Create project and init deploy
	mustRun(t, "bffx", "new", "test-stack", "--root", root)
	projectDir := filepath.Join(root, "test-stack")
	mustRun(t, "bffx", "deploy", "init", "--root", projectDir)

	// Vendor the current local bffx copy into the scaffolded project
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	workspaceRoot := filepath.Dir(filepath.Dir(wd))
	if err := deploy.Vendor(projectDir, workspaceRoot, nil); err != nil {
		t.Fatalf("vendor bffx failed: %v", err)
	}

	// docker compose up -d (detached)
	// Avoid ambiguous multi-file Compose projects and stale scaffolds: deploy uses docker-compose.yml only.
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "up", "-d", "--build")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		cmdLogs := exec.Command("docker", "compose", "-f", "docker-compose.yml", "logs")
		cmdLogs.Dir = projectDir
		logsOut, _ := cmdLogs.CombinedOutput()
		t.Fatalf("docker compose up failed: %v\n%s\n--- CONTAINER LOGS ---\n%s", err, out, logsOut)
	}

	// Ensure we clean up the containers
	t.Cleanup(func() {
		cmdDown := exec.Command("docker", "compose", "-f", "docker-compose.yml", "down", "-v")
		cmdDown.Dir = projectDir
		_ = cmdDown.Run()
	})

	// 3. Poll /health until ready (max 180s)
	url := "http://localhost:8080/health"
	deadline := time.Now().Add(180 * time.Second)
	healthy := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			t.Logf("✅ Stack healthy at %s", url)
			healthy = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !healthy {
		cmdLogs := exec.Command("docker", "compose", "-f", "docker-compose.yml", "logs")
		cmdLogs.Dir = projectDir
		logsOut, _ := cmdLogs.CombinedOutput()
		t.Fatalf("stack never became healthy at %s within 180s\n--- CONTAINER LOGS ---\n%s", url, logsOut)
	}

	// Run nested subtests now while the stack is still up
	t.Run("BootstrapEndpointResponds", func(t *testing.T) {
		url := "http://localhost:8080/api/v1/app/bootstrap"
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("X-App-Secret", "bffx-dev-app-secret-key-must-be-at-least-16-characters")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("bootstrap request failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("RedisContainerIsHealthy", func(t *testing.T) {
		cmdPing := exec.Command("docker", "compose", "-f", "docker-compose.yml", "exec", "-T", "redis", "redis-cli", "ping")
		cmdPing.Dir = projectDir
		out, err := cmdPing.CombinedOutput()
		if err != nil {
			t.Fatalf("redis ping failed: %v\n%s", err, out)
		}
		if string(out) != "PONG\n" {
			t.Errorf("expected PONG, got %q", string(out))
		}
	})
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if name == "bffx" {
		wd, _ := os.Getwd()
		repoRoot := filepath.Dir(filepath.Dir(wd))
		name = filepath.Join(repoRoot, "bffx")
		// Use a CLI built from this checkout (avoids stale ./bffx after deploy template changes).
		build := exec.Command("go", "build", "-o", name, "./cmd/bffx")
		build.Dir = repoRoot
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("rebuild bffx CLI: %v\n%s", err, out)
		}
	}
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

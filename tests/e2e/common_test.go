package e2e

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testMode = flag.String("mode", "host", "test mode: host or docker")

type E2ESpec struct {
	Name         string
	ScaffoldArgs []string
	Port         int
	HostTests    func(t *testing.T, port int)
	DockerTests  func(t *testing.T, port int)
}

func RunEphemeralTest(t *testing.T, spec E2ESpec) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	mode := *testMode
	t.Logf("🧪 Running in MODE: %s", mode)

	// 1. Initialize Paths
	wd, _ := os.Getwd()
	root, err := bffxModuleRoot()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	bffxBin, err := bffxCLIPath()
	if err != nil {
		t.Fatalf("bffx cli: %v", err)
	}
	testProjectsDir := filepath.Join(root, "test-projects")
	projectDir := filepath.Join(testProjectsDir, spec.Name)
	logsDir := filepath.Join(wd, "logs")

	// 2. Initialize Logging & Workspaces
	os.MkdirAll(logsDir, 0o755)
	os.MkdirAll(testProjectsDir, 0o755)
	timestamp := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logsDir, fmt.Sprintf("%s_%s_%s.log", timestamp, mode, spec.Name))
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	logWriter := io.Writer(logFile)
	t.Logf("📝 Logging to %s", logPath)

	// 3. Targeted Cleanup
	os.RemoveAll(projectDir)

	// 4. Scaffold
	t.Log("🏗️ Scaffolding project...")
	scaffoldArgs := append([]string{"new", spec.Name, "--non-interactive", "--dev-vendor"}, spec.ScaffoldArgs...)
	scaffold := exec.Command(bffxBin, scaffoldArgs...)
	scaffold.Dir = testProjectsDir
	scaffold.Stdout = logWriter
	scaffold.Stderr = logWriter
	if err := scaffold.Run(); err != nil {
		t.Fatalf("scaffold failed: %v (see logs at %s)", err, logPath)
	}

	// Remove BFFX_APP_SECRET from scaffolded .env so it doesn't block API requests in E2E tests
	envPath := filepath.Join(projectDir, ".env")
	if data, err := os.ReadFile(envPath); err == nil {
		lines := strings.Split(string(data), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.HasPrefix(line, "BFFX_APP_SECRET=") {
				newLines = append(newLines, line)
			}
		}
		os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")), 0644)
	}

	if mode == "docker" {
		t.Log("🐙 deploy init (Dockerfile + docker-compose.yml)...")
		di := exec.Command(bffxBin, "deploy", "init", "--root", projectDir)
		di.Stdout = logWriter
		di.Stderr = logWriter
		if err := di.Run(); err != nil {
			t.Fatalf("deploy init failed: %v (see logs at %s)", err, logPath)
		}
	}

	if mode == "host" {
		// 5. STAGE 1: HOST VERIFICATION
		t.Log("🚀 STAGE 1: HOST VERIFICATION")
		entry := "./cmd/api/main.go"
		binName := "api-server"
		if _, err := os.Stat(filepath.Join(projectDir, "cmd", "orchestrator", "main.go")); err == nil {
			entry = "./cmd/orchestrator/main.go"
			binName = "orchestrator"
		}
		build := exec.Command("go", "build", "-o", binName, entry)
		build.Dir = projectDir
		build.Stdout = logWriter
		build.Stderr = logWriter
		if err := build.Run(); err != nil {
			t.Fatalf("host build failed: %v", err)
		}

		hostApi := exec.Command("./" + binName)
		hostApi.Dir = projectDir
		hostApi.Env = append(os.Environ(), "BFFX_JWT_SECRET=test-secret-at-least-32-chars-long-!!!", fmt.Sprintf("PORT=%d", spec.Port))
		hostApi.Stdout = logWriter
		hostApi.Stderr = logWriter
		if err := hostApi.Start(); err != nil {
			t.Fatalf("failed to start host api: %v", err)
		}

		hostKilled := false
		killHost := func() {
			if !hostKilled {
				hostApi.Process.Kill()
				hostKilled = true
			}
		}
		defer killHost()

		waitForHealthy(t, fmt.Sprintf("http://localhost:%d/health", spec.Port))

		if spec.HostTests != nil {
			spec.HostTests(t, spec.Port)
		}

		killHost()
		time.Sleep(1 * time.Second)
	} else if mode == "docker" {
		// 6. STAGE 2: DOCKER VERIFICATION
		t.Log("🐳 STAGE 2: DOCKER VERIFICATION")

		dcPath := filepath.Join(projectDir, "docker-compose.yml")
		if _, err := os.Stat(dcPath); err != nil {
			dcPath = filepath.Join(projectDir, "docker-compose.yaml")
			if _, err := os.Stat(dcPath); err != nil {
				t.Fatalf("docker compose file missing (expected docker-compose.yml from deploy init or legacy docker-compose.yaml): %v", err)
			}
		}
		dcData, _ := os.ReadFile(dcPath)
		dcContent := strings.Replace(string(dcData), "8080:8080", fmt.Sprintf("%d:8080", spec.Port), 1)
		os.WriteFile(dcPath, []byte(dcContent), 0o644)

		composeBase := filepath.Base(dcPath)

		dockerUp := exec.Command("docker", "compose", "-f", composeBase, "up", "-d", "--build")
		dockerUp.Dir = projectDir
		dockerUp.Stdout = logWriter
		dockerUp.Stderr = logWriter
		if err := dockerUp.Run(); err != nil {
			t.Fatalf("docker compose up failed: %v", err)
		}

		defer func() {
			t.Log("🧹 Cleaning up Docker...")
			down := exec.Command("docker", "compose", "-f", composeBase, "down", "-v")
			down.Dir = projectDir
			down.Stdout = logWriter
			down.Stderr = logWriter
			down.Run()
		}()

		waitForHealthy(t, fmt.Sprintf("http://localhost:%d/health", spec.Port))

		if spec.DockerTests != nil {
			spec.DockerTests(t, spec.Port)
		}
	}

	// 7. Cleanup Project Folder
	t.Log("🧹 Removing test project folder...")
	defer os.RemoveAll(projectDir)

	t.Logf("✨ Ephemeral test %q in mode %s passed!", spec.Name, mode)
}

func waitForHealthy(t *testing.T, url string) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Service at %s failed to become healthy", url)
}

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployInitOutput(t *testing.T) {
	root := t.TempDir()

	// 1. Scaffold a project
	projectName := "test-deploy"
	mustRun(t, "bffx", "new", projectName, "--root", root, "--non-interactive")
	projectDir := filepath.Join(root, projectName)

	// 2. Run deploy init
	mustRun(t, "bffx", "deploy", "init", "--root", projectDir)

	// 3. Verify Dockerfile
	dfContent, err := os.ReadFile(filepath.Join(projectDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("Dockerfile missing: %v", err)
	}
	dfStr := string(dfContent)
	if !strings.Contains(dfStr, "golang:1.26-alpine") {
		t.Errorf("expected Go 1.26 in Dockerfile, got: %s", dfStr)
	}
	if !strings.Contains(dfStr, "--mount=type=cache") {
		t.Errorf("Dockerfile missing BuildKit cache mounts")
	}

	// 4. Verify Docker Compose variants
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.staging.yml",
		"docker-compose.prod.yml",
	}
	for _, f := range composeFiles {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("%s missing", f)
		}
	}

	// 5. Verify scripts
	if _, err := os.Stat(filepath.Join(projectDir, "scripts", "setup-droplet.sh")); err != nil {
		t.Errorf("setup-droplet.sh missing")
	}

	// 6. Verify .dockerignore
	diContent, _ := os.ReadFile(filepath.Join(projectDir, ".dockerignore"))
	if !strings.Contains(string(diContent), ".env") {
		t.Errorf(".dockerignore missing .env entry")
	}
}

func TestGenerateCICDOutput(t *testing.T) {
	root := t.TempDir()
	projectName := "test-cicd"
	mustRun(t, "bffx", "new", projectName, "--root", root, "--non-interactive")
	projectDir := filepath.Join(root, projectName)

	mustRun(t, "bffx", "generate", "cicd", "--root", projectDir)

	workflowPath := filepath.Join(projectDir, ".github", "workflows", "deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("deploy.yml missing: %v", err)
	}

	workflowStr := string(content)
	if !strings.Contains(workflowStr, "go-version: '1.26'") {
		t.Errorf("expected Go 1.26 in CI workflow")
	}
	if !strings.Contains(workflowStr, "ghcr.io") {
		t.Errorf("expected GHCR reference in CI workflow")
	}
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if name == "bffx" {
		path, err := bffxCLIPath()
		if err != nil {
			t.Fatalf("bffx cli: %v", err)
		}
		name = path
	}
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

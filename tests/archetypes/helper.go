package archetypes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/deploy"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// SetupArchetypeProject scaffolds the given archetype and vendors the framework so it is compileable.
func SetupArchetypeProject(t *testing.T, tmpDir, alias string) string {
	t.Helper()
	projectDir := filepath.Join(tmpDir, alias)

	// 1. Scaffold the archetype
	err := generator.ScaffoldArchetype(tmpDir, alias, alias, generator.ProjectOptions{})
	if err != nil {
		t.Fatalf("failed to scaffold archetype %s: %v", alias, err)
	}

	// 2. Vendor local framework
	workspaceRoot := deploy.DetectWorkspaceRoot(projectDir)
	if workspaceRoot == "" {
		t.Fatal("could not detect framework workspace root")
	}

	err = deploy.Vendor(projectDir, workspaceRoot, nil)
	if err != nil {
		t.Fatalf("failed to vendor local framework: %v", err)
	}

	// Remove BFFX_APP_SECRET from scaffolded .env so it doesn't block API requests in archetype tests
	envPath := filepath.Join(projectDir, ".env")
	if data, err := os.ReadFile(envPath); err == nil {
		lines := strings.Split(string(data), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.HasPrefix(line, "BFFX_APP_SECRET=") {
				newLines = append(newLines, line)
			}
		}
		_ = os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")), 0644)
	}

	return projectDir
}

// SyncArchetypeProject runs compiler sync on the project directory.
func SyncArchetypeProject(t *testing.T, projectDir string) {
	t.Helper()

	// Parse project.yaml and temporarily swap postgres for sqlite so sync/compile
	// succeeds offline without a running Postgres database.
	projPath := filepath.Join(projectDir, "bffx", "project.yaml")
	if data, err := os.ReadFile(projPath); err == nil {
		content := string(data)
		changed := false
		if strings.Contains(content, "mode: postgres") {
			content = strings.ReplaceAll(content, "mode: postgres", "mode: sqlite")
			changed = true
		}
		if strings.Contains(content, "store: postgres") {
			content = strings.ReplaceAll(content, "store: postgres", "store: sqlite")
			changed = true
		}
		if changed {
			if err := os.WriteFile(projPath, []byte(content), 0o644); err != nil {
				t.Fatalf("rewrite project.yaml for offline sync: %v", err)
			}
			manifest.InvalidateLoadAllCache(projectDir)
		}
	}

	if _, err := compiler.Sync(projectDir, false); err != nil {
		t.Fatalf("bffx sync failed: %v", err)
	}
}

// BuildArchetypeProject compiles the project's orchestrator command and asserts it succeeds.
func BuildArchetypeProject(t *testing.T, projectDir string, layout string) {
	t.Helper()

	cmdName := "orchestrator"
	if _, err := os.Stat(filepath.Join(projectDir, "cmd", "api")); err == nil {
		cmdName = "api"
	}
	outBin := filepath.Join(projectDir, ".bffx", "orchestrator.test.bin")
	_ = os.MkdirAll(filepath.Dir(outBin), 0o755)

	cmd := exec.Command("go", "build", "-o", outBin, "./cmd/"+cmdName)
	cmd.Dir = projectDir
	cmd.Env = compiler.GoBuildEnv()
	if bout, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile cmd/%s: %v\n%s", cmdName, err, bout)
	}
}

package notes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/compiler"
)

func notesExampleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 12 {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if strings.Contains(string(b), "module bffx/examples/notes") {
				if _, err := os.Stat(filepath.Join(dir, "bffx", "project.yaml")); err == nil {
					return dir
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate examples/notes (module bffx/examples/notes + bffx/project.yaml)")
	return ""
}

func TestBffxSyncAndOrchestratorBuild(t *testing.T) {
	root := notesExampleRoot(t)
	if _, err := compiler.Sync(root, false); err != nil {
		t.Fatalf("bffx sync: %v", err)
	}
	regPath := filepath.Join(root, "cmd", "api", "registry.gen.go")
	if _, err := os.Stat(regPath); err != nil {
		t.Fatalf("missing generated registry: %v", err)
	}
	out := filepath.Join(root, ".bffx", "api.test.bin")
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	cmd := exec.Command("go", "test", "-c", "-o", out, "./cmd/api")
	cmd.Dir = root
	cmd.Env = compiler.GoBuildEnv()
	if bout, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile ./cmd/api: %v\n%s", err, bout)
	}
}

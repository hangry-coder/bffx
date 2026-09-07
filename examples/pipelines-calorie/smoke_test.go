package pipelinescalorie

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func exampleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 12 {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if strings.Contains(string(b), "module bffx/examples/pipelines-calorie") {
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
	t.Fatal("could not locate examples/pipelines-calorie")
	return ""
}

func TestLoadAll_DiscoversPipeline(t *testing.T) {
	root := exampleRoot(t)
	reg, err := manifest.LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(reg.Pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(reg.Pipelines))
	}
	if reg.Pipelines[0].Metadata.Name != "MealVision" {
		t.Fatalf("pipeline name = %q", reg.Pipelines[0].Metadata.Name)
	}
}

func TestBffxSyncAndOrchestratorBuild(t *testing.T) {
	root := exampleRoot(t)
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

func TestProjectYAML_NoNutritionBattery(t *testing.T) {
	root := exampleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "bffx", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "nutrition:") {
		t.Fatal("pipelines-calorie example must not use batteries.nutrition; use pipeline catalog adapter")
	}
}

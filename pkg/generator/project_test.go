package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldNewProject(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-gen-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	appName := "mybase"
	opts := ProjectOptions{
		StoreMode:     "sqlite",
		WithTelemetry: true,
	}

	if err := ScaffoldNewProject(tmp, appName, opts); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	appRoot := filepath.Join(tmp, appName)
	expectedDirs := []string{
		"internal/features",
		"cmd/api",
		"db/migrations",
		"worker/skills",
	}

	for _, d := range expectedDirs {
		if _, err := os.Stat(filepath.Join(appRoot, d)); os.IsNotExist(err) {
			t.Errorf("expected directory %s missing", d)
		}
	}

	// Verify project.yaml
	projPath := filepath.Join(appRoot, "bffx/project.yaml")
	if _, err := os.Stat(projPath); os.IsNotExist(err) {
		t.Errorf("project.yaml missing")
	}

	content, _ := os.ReadFile(projPath)
	if !strings.Contains(string(content), "telemetryStore:") {
		t.Errorf("expected telemetryStore in project.yaml")
	}
}

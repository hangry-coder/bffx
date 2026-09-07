package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldNewProjectLegacyLayout(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-gen-legacy-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	appName := "legacyapp"
	opts := ProjectOptions{
		Layout:    LayoutLegacy,
		StoreMode: "sqlite",
	}

	if err := ScaffoldNewProject(tmp, appName, opts); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	appRoot := filepath.Join(tmp, appName)
	if _, err := os.Stat(filepath.Join(appRoot, "bffx/resources")); err != nil {
		t.Errorf("expected legacy bffx/resources: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "cmd/orchestrator")); err != nil {
		t.Errorf("expected legacy cmd/orchestrator: %v", err)
	}
}

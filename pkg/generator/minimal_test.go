package generator

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Regenerate: go test ./pkg/generator -run TestMinimalScaffold_FileTreeGolden -update-minimal-golden
var updateMinimalGolden = flag.Bool("update-minimal-golden", false, "rewrite testdata/golden/new_minimal_paths.golden from current minimal scaffold")

func minimalScaffoldRelPaths(projectDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func TestMinimalScaffold_FileTreeGolden(t *testing.T) {
	tmp := t.TempDir()
	appName := "goldenminimal"
	opts := ProjectOptions{
		Minimal:       true,
		StoreMode:     "sqlite",
		WithTelemetry: false,
	}
	if err := ScaffoldNewProject(tmp, appName, opts); err != nil {
		t.Fatalf("ScaffoldNewProject: %v", err)
	}
	appRoot := filepath.Join(tmp, appName)
	paths, err := minimalScaffoldRelPaths(appRoot)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(paths, "\n") + "\n"

	goldenPath := filepath.Join("testdata", "golden", "new_minimal_paths.golden")
	if *updateMinimalGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("updated", goldenPath)
		return
	}
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(wantBytes) != got {
		t.Errorf("minimal scaffold file tree drifted.\nRun: go test ./pkg/generator -run TestMinimalScaffold_FileTreeGolden -update-minimal-golden\n--- golden\n%s\n--- got\n%s", wantBytes, got)
	}
}

func TestMinimalScaffold_OmitsFullOnlyArtifacts(t *testing.T) {
	tmp := t.TempDir()
	appName := "minapp"
	opts := ProjectOptions{Minimal: true, StoreMode: "sqlite", WithTelemetry: false}
	if err := ScaffoldNewProject(tmp, appName, opts); err != nil {
		t.Fatalf("ScaffoldNewProject: %v", err)
	}
	root := filepath.Join(tmp, appName)
	fullOnly := []string{
		"bffx/actions/verify.yaml",
		"bffx/screens/devices.yaml",
	}
	for _, rel := range fullOnly {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("minimal scaffold should not include %s", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("stat %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
		t.Errorf("README.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/features/system/manifests/user.yaml")); err != nil {
		t.Errorf("user resource: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "bffx dev") || !strings.Contains(string(readme), "License") {
		t.Errorf("README missing quickstart or license section")
	}
}

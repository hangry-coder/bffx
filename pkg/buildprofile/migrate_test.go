package buildprofile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestPackagingMigration_fullToMinimalRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	appName := "pack-migrate"
	if err := generator.ScaffoldNewProject(tmp, appName, generator.ProjectOptions{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, appName)

	reg, err := manifest.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if buildprofile.ModeFromSpec(reg.ProjectSpec()) != buildprofile.ModeFull {
		t.Fatalf("expected full mode by default, got %q", buildprofile.ModeFromSpec(reg.ProjectSpec()))
	}

	plan, err := buildprofile.PlanMigration(reg, buildprofile.ModeMinimal)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ToMode != buildprofile.ModeMinimal {
		t.Fatalf("plan=%+v", plan)
	}

	applied, err := buildprofile.ApplyMigration(root, reg, buildprofile.ModeMinimal, false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.FromMode != buildprofile.ModeFull || applied.ToMode != buildprofile.ModeMinimal {
		t.Fatalf("applied=%+v", applied)
	}

	reg, err = manifest.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if buildprofile.ModeFromSpec(reg.ProjectSpec()) != buildprofile.ModeMinimal {
		t.Fatal("project.yaml not updated to minimal")
	}

	prof, err := buildprofile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if prof.Mode != buildprofile.ModeMinimal {
		t.Fatalf("profile mode=%q", prof.Mode)
	}
	if len(prof.Packages) == 0 {
		t.Fatal("minimal profile should list runtime packages")
	}

	rec, err := buildprofile.LoadMigrationRecord(root)
	if err != nil || rec == nil {
		t.Fatalf("migration record missing: %v", err)
	}

	if err := buildprofile.RollbackMigration(root); err != nil {
		t.Fatal(err)
	}
	reg, err = manifest.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if buildprofile.ModeFromSpec(reg.ProjectSpec()) != buildprofile.ModeFull {
		t.Fatal("rollback did not restore full mode")
	}
	recAfter, err := buildprofile.LoadMigrationRecord(root)
	if err != nil {
		t.Fatal(err)
	}
	if recAfter != nil {
		t.Fatal("migration record should be cleared after rollback")
	}
}

func TestPackagingMigration_dryRunDoesNotMutate(t *testing.T) {
	tmp := t.TempDir()
	appName := "pack-dry"
	if err := generator.ScaffoldNewProject(tmp, appName, generator.ProjectOptions{Minimal: false}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, appName)
	reg, err := manifest.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(filepath.Join(root, "bffx", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := buildprofile.ApplyMigration(root, reg, buildprofile.ModeMinimal, true); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(filepath.Join(root, "bffx", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run should not modify project.yaml")
	}
}

func TestPackagingMigrationMatrix_minimalScaffoldProfile(t *testing.T) {
	tmp := t.TempDir()
	fullName := "matrix-full"
	minName := "matrix-min"

	if err := generator.ScaffoldNewProject(tmp, fullName, generator.ProjectOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := generator.ScaffoldNewProject(tmp, minName, generator.ProjectOptions{Minimal: true}); err != nil {
		t.Fatal(err)
	}

	fullRoot := filepath.Join(tmp, fullName)
	minRoot := filepath.Join(tmp, minName)

	fullReg, err := manifest.LoadAll(fullRoot)
	if err != nil {
		t.Fatal(err)
	}
	minReg, err := manifest.LoadAll(minRoot)
	if err != nil {
		t.Fatal(err)
	}

	fullProf, err := buildprofile.Derive(fullReg, buildprofile.DeriveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	minProf, err := buildprofile.Derive(minReg, buildprofile.DeriveOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if fullProf.Mode != buildprofile.ModeFull {
		t.Fatalf("full mode=%q", fullProf.Mode)
	}
	if minProf.Mode != buildprofile.ModeMinimal {
		t.Fatalf("minimal mode=%q", minProf.Mode)
	}
	if len(minProf.Packages) == 0 {
		t.Fatal("minimal profile must enumerate runtime packages")
	}
	if len(minProf.ExcludedTooling) != len(buildprofile.ToolingPackages) {
		t.Fatalf("excluded tooling=%d want %d", len(minProf.ExcludedTooling), len(buildprofile.ToolingPackages))
	}
	if len(fullProf.Packages) != 0 {
		t.Fatalf("full profile packages should be empty/nil (copy all), got %d", len(fullProf.Packages))
	}

	yamlBytes, err := os.ReadFile(filepath.Join(minRoot, "bffx", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(yamlBytes), "mode: minimal") {
		t.Fatal("minimal scaffold should emit packaging.mode in project.yaml")
	}
}

func TestPreflight_alreadyAtTarget(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Spec: specNode(t, map[string]any{
				"packaging": map[string]any{"mode": "minimal"},
			}),
		},
	}
	pf := buildprofile.Preflight(reg, buildprofile.ModeMinimal)
	if pf.OK {
		t.Fatal("expected blocker when already minimal")
	}
}

func specNode(t *testing.T, spec map[string]any) yaml.Node {
	t.Helper()
	b, err := yaml.Marshal(map[string]any{"spec": spec})
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m.Spec
}

package manifest_test

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"os"
	"path/filepath"
	"testing"
)

func setupTempProject(t *testing.T, flagDirs []string) string {
	t.Helper()
	dir := t.TempDir()
	bffxDir := filepath.Join(dir, "bffx")
	err := os.MkdirAll(bffxDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create project file
	projYaml := `apiVersion: v1
kind: Project
metadata:
  name: test-project
spec:
  runtime:
    api:
      language: go
`
	err = os.WriteFile(filepath.Join(bffxDir, "project.yaml"), []byte(projYaml), 0644)
	if err != nil {
		t.Fatal(err)
	}

	for i, flagDir := range flagDirs {
		fullFlagDir := filepath.Join(bffxDir, flagDir)
		err = os.MkdirAll(fullFlagDir, 0755)
		if err != nil {
			t.Fatal(err)
		}

		flagYaml := `apiVersion: v1
kind: FeatureFlag
metadata:
  name: flag-` + string(rune('a'+i)) + `
spec:
  key: test-flag-` + string(rune('a'+i)) + `
  enabled: true
  variations: [true, false]
  fallthrough:
    variation: 0
  offVariation: 1
`
		err = os.WriteFile(filepath.Join(fullFlagDir, "flag.yaml"), []byte(flagYaml), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestManifestDiscovery_FeatureFlagInFeaturesDir(t *testing.T) {
	dir := setupTempProject(t, []string{"features"})
	
	reg, err := manifest.LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(reg.FeatureFlags) != 1 {
		t.Fatalf("Expected 1 FeatureFlag, got %d", len(reg.FeatureFlags))
	}
	
	if reg.FeatureFlags[0].Metadata.Name != "flag-a" {
		t.Errorf("Expected flag-a, got %s", reg.FeatureFlags[0].Metadata.Name)
	}
}

func TestManifestDiscovery_FeatureFlagInFlagsDir(t *testing.T) {
	dir := setupTempProject(t, []string{"flags"})
	
	reg, err := manifest.LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(reg.FeatureFlags) != 1 {
		t.Fatalf("Expected 1 FeatureFlag, got %d", len(reg.FeatureFlags))
	}
	
	if reg.FeatureFlags[0].Metadata.Name != "flag-a" {
		t.Errorf("Expected flag-a, got %s", reg.FeatureFlags[0].Metadata.Name)
	}
}

func TestManifestDiscovery_FeatureFlagInBothDirs(t *testing.T) {
	dir := setupTempProject(t, []string{"features", "flags"})
	
	reg, err := manifest.LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(reg.FeatureFlags) != 2 {
		t.Fatalf("Expected 2 FeatureFlags, got %d", len(reg.FeatureFlags))
	}
}

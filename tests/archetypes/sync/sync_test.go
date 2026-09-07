package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/generator/archetypes"
	testHelper "github.com/hangry-coder/bffx/tests/archetypes"
)

func TestArchetypeSync(t *testing.T) {
	registry := archetypes.GetRegistry()
	if registry == nil || len(registry.Archetypes) == 0 {
		t.Fatal("empty archetype registry")
	}

	for _, arch := range registry.Archetypes {
		t.Run(arch.Name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "bffx-sync-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			projectDir := testHelper.SetupArchetypeProject(t, tmpDir, arch.Name)
			testHelper.SyncArchetypeProject(t, projectDir)

			// The BFFX compiler generates registry.gen.go in cmd/api (v2) or cmd/orchestrator (legacy)
			cmdDirName := "orchestrator"
			if _, err := os.Stat(filepath.Join(projectDir, "cmd", "api")); err == nil {
				cmdDirName = "api"
			}
			regPath := filepath.Join(projectDir, "cmd", cmdDirName, "registry.gen.go")
			if _, err := os.Stat(regPath); os.IsNotExist(err) {
				t.Errorf("expected generated registry missing: %s", regPath)
			}
		})
	}
}

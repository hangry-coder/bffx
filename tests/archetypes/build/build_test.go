package build

import (
	"os"
	"testing"

	"github.com/hangry-coder/bffx/pkg/generator/archetypes"
	testHelper "github.com/hangry-coder/bffx/tests/archetypes"
)

func TestArchetypeBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heavy build tests in short mode")
	}

	registry := archetypes.GetRegistry()
	if registry == nil || len(registry.Archetypes) == 0 {
		t.Fatal("empty archetype registry")
	}

	for _, arch := range registry.Archetypes {
		t.Run(arch.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir, err := os.MkdirTemp("", "bffx-build-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			projectDir := testHelper.SetupArchetypeProject(t, tmpDir, arch.Name)
			testHelper.SyncArchetypeProject(t, projectDir)
			testHelper.BuildArchetypeProject(t, projectDir, arch.Layout)
		})
	}
}

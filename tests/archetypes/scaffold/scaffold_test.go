package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/generator/archetypes"
)

func TestArchetypeScaffolding(t *testing.T) {
	registry := archetypes.GetRegistry()
	if registry == nil || len(registry.Archetypes) == 0 {
		t.Fatal("empty archetype registry")
	}

	for _, arch := range registry.Archetypes {
		t.Run(arch.Name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "bffx-scaffold-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// 1. Run scaffolding
			err = generator.ScaffoldArchetype(tmpDir, arch.Name, arch.Name, generator.ProjectOptions{})
			if err != nil {
				t.Fatalf("failed to scaffold archetype %s: %v", arch.Name, err)
			}

			projectDir := filepath.Join(tmpDir, arch.Name)

			// 2. Validate directory layout shape
			var expectedDirs []string
			if arch.Layout == string(generator.LayoutV2) {
				expectedDirs = []string{
					"internal/features",
					"internal/platform",
					"config",
					"cmd/api",
					"cmd/worker",
				}
			} else {
				expectedDirs = []string{
					"bffx/resources",
					"bffx/builders",
					"hooks",
				}
			}

			for _, dir := range expectedDirs {
				path := filepath.Join(projectDir, dir)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("expected directory %s missing in %s layout", dir, arch.Layout)
				}
			}

			// 3. Read project.yaml and verify
			projPath := filepath.Join(projectDir, "bffx/project.yaml")
			contentBytes, err := os.ReadFile(projPath)
			if err != nil {
				t.Fatalf("failed to read project.yaml: %v", err)
			}
			content := string(contentBytes)

			// Assert layout in project.yaml matches
			if !strings.Contains(content, "layout: "+arch.Layout) {
				t.Errorf("expected project.yaml to contain 'layout: %s'", arch.Layout)
			}

			// Assert store mode in project.yaml matches
			if !strings.Contains(content, "mode: "+arch.Batteries.Store) {
				t.Errorf("expected project.yaml to contain 'mode: %s'", arch.Batteries.Store)
			}

			// Anti-drift check: No "nutrition:" in project.yaml
			if strings.Contains(content, "nutrition:") {
				t.Errorf("anti-drift violation: 'nutrition:' battery found in project.yaml")
			}

			// 4. Validate post-scaffold AI pipeline files
			if arch.Pipeline != nil {
				feature := arch.Pipeline.Feature
				if feature == "" {
					feature = "app"
				}
				
				// Verify pipeline manifest
				var manifestPath string
				if arch.Layout == string(generator.LayoutV2) {
					manifestPath = filepath.Join(projectDir, "internal", "features", feature, "manifests", strings.ToLower(arch.Pipeline.Name)+"_pipeline.yaml")
				} else {
					manifestPath = filepath.Join(projectDir, "bffx", "resources", strings.ToLower(arch.Pipeline.Name)+"_pipeline.yaml")
				}

				if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
					t.Errorf("expected pipeline manifest missing: %s", manifestPath)
				}

				// Verify pipeline Go code stubs in internal/features/<feature>/pipelines/<name>/
				pipelineGoPath := filepath.Join(projectDir, "internal", "features", feature, "pipelines", strings.ToLower(arch.Pipeline.Name), "pipeline.go")
				if _, err := os.Stat(pipelineGoPath); os.IsNotExist(err) {
					t.Errorf("expected pipeline Go file missing: %s", pipelineGoPath)
				}

				pipelineGoContent, err := os.ReadFile(pipelineGoPath)
				if err != nil {
					t.Fatalf("failed to read pipeline Go file: %v", err)
				}

				expectedPackageName := "package " + strings.ToLower(arch.Pipeline.Name)
				if !strings.Contains(string(pipelineGoContent), expectedPackageName) {
					t.Errorf("expected pipeline Go file to belong to package %s", strings.ToLower(arch.Pipeline.Name))
				}
			}
		})
	}
}

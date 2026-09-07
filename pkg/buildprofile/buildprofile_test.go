package buildprofile

import (
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestDerive_minimalModeAndCapabilities(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "demo"},
			Spec: mustYAMLNode(t, map[string]any{
				"packaging": map[string]any{"mode": "minimal"},
				"admin":     map[string]any{"enabled": true},
				"runtime": map[string]any{
					"wire":      map[string]any{"enabled": true},
					"streaming": map[string]any{"enabled": true},
					"worker":    map[string]any{"enabled": true},
				},
				"batteries": map[string]any{
					"flags": "bffx",
					"vlm":   "gemini",
				},
			}),
		},
		Pipelines: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "chat"}},
		},
		LiveOpsEvents: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "season"}},
		},
	}

	prof, err := Derive(reg, DeriveOptions{GraphHash: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if prof.Mode != ModeMinimal {
		t.Fatalf("mode=%q want minimal", prof.Mode)
	}
	if !prof.Capabilities.AI || !prof.Capabilities.Game || !prof.Capabilities.Admin {
		t.Fatalf("capabilities=%+v", prof.Capabilities)
	}
	if prof.GraphHash != "abc123" {
		t.Fatalf("graphHash=%q", prof.GraphHash)
	}
	if len(prof.Packages) == 0 {
		t.Fatal("expected minimal package list")
	}
	for _, p := range ToolingPackages {
		for _, included := range prof.Packages {
			if included == p {
				t.Fatalf("tooling package %q must not appear in minimal packages", p)
			}
		}
	}
	if len(prof.ExcludedTooling) != len(ToolingPackages) {
		t.Fatalf("excludedTooling=%v", prof.ExcludedTooling)
	}
}

func TestValidateConsistency_pipelineWithoutAI(t *testing.T) {
	prof := &Profile{
		Counts:       ManifestCounts{Pipelines: 1},
		Capabilities: Capabilities{AI: false},
	}
	issues := ValidateConsistency(prof, &manifest.Registry{})
	if len(issues) == 0 {
		t.Fatal("expected consistency issue")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	prof := &Profile{
		APIVersion: APIVersion,
		Project:    "demo",
		Mode:       ModeFull,
		Capabilities: Capabilities{
			FeatureFlags: true,
		},
	}
	prof.ProfileHash = hashProfile(prof)
	if err := Write(root, prof); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project != "demo" || loaded.Mode != ModeFull {
		t.Fatalf("loaded=%+v", loaded)
	}
}

func TestDriftIssues_staleProfile(t *testing.T) {
	stored := &Profile{ProfileHash: "old", GraphHash: "g1", Mode: ModeFull}
	derived := &Profile{ProfileHash: "new", GraphHash: "g2", Mode: ModeMinimal}
	issues := DriftIssues(stored, derived)
	if len(issues) < 2 {
		t.Fatalf("issues=%v", issues)
	}
}

func mustYAMLNode(t *testing.T, spec map[string]any) yaml.Node {
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

func TestPath(t *testing.T) {
	got := Path(filepath.Join("proj"))
	if got != filepath.Join("proj", ".bffx", Filename) {
		t.Fatalf("path=%q", got)
	}
}

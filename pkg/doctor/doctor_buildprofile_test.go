package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestLintBuildProfile_missingProfile(t *testing.T) {
	root := t.TempDir()
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "demo"}},
	}
	got := lintBuildProfile(root, reg)
	if len(got) != 1 || got[0].Status != "warn" {
		t.Fatalf("got=%+v", got)
	}
}

func TestLintBuildProfile_ok(t *testing.T) {
	root := t.TempDir()
	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "demo"},
			Spec:     projectSpecNode(t, map[string]any{"admin": map[string]any{"enabled": true}}),
		},
	}
	prof, err := buildprofile.Derive(reg, buildprofile.DeriveOptions{GraphHash: "deadbeef"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".bffx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := buildprofile.Write(root, prof); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".bffx", "graph.hash"), []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := lintBuildProfile(root, reg)
	for _, r := range got {
		if r.Status == "fail" {
			t.Fatalf("unexpected fail: %+v", got)
		}
	}
}

func projectSpecNode(t *testing.T, spec map[string]any) yaml.Node {
	t.Helper()
	wrapped := map[string]any{"spec": spec}
	b, err := yaml.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m.Spec
}

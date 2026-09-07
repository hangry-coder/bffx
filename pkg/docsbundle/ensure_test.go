package docsbundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBundled(t *testing.T) {
	framework := t.TempDir()
	project := t.TempDir()
	docsRoot := filepath.Join(framework, "docs", "getting-started")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "quickstart.md"), []byte("# Quickstart\n\nHello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(framework, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	bffxMeta := filepath.Join(project, ".bffx")
	if err := os.MkdirAll(bffxMeta, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bffxMeta, "framework_root"), []byte(framework+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureBundled(project); err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(project, ".bffx", "docs", "index.json")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("expected index after ensure: %v", err)
	}
}

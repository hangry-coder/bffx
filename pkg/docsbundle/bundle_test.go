package docsbundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundle(t *testing.T) {
	framework := t.TempDir()
	project := t.TempDir()
	docsRoot := filepath.Join(framework, "docs", "getting-started")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "quickstart.md"), []byte("# Quickstart\n\nHello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Bundle(project, framework); err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(project, ".bffx", "docs", "index.json")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	got := filepath.Join(project, ".bffx", "docs", "getting-started", "quickstart.md")
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("bundled file: %v", err)
	}
}

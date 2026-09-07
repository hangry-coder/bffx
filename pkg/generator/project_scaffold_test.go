package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldDirectoriesLegacyLayout(t *testing.T) {
	dir := t.TempDir()
	if err := scaffoldDirectories(dir, ProjectOptions{Layout: LayoutLegacy}); err != nil {
		t.Fatalf("scaffoldDirectories: %v", err)
	}
	for _, sub := range []string{"bffx/resources", "hooks", "cmd/orchestrator", ".bffx/framework_version"} {
		p := filepath.Join(dir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %q to exist: %v", sub, err)
		}
	}
}

func TestScaffoldDirectoriesV2Layout(t *testing.T) {
	dir := t.TempDir()
	if err := scaffoldDirectories(dir, ProjectOptions{Layout: LayoutV2}); err != nil {
		t.Fatalf("scaffoldDirectories: %v", err)
	}
	for _, sub := range []string{"internal/features", "db/migrations", "cmd/api", "assets/i18n"} {
		p := filepath.Join(dir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %q to exist: %v", sub, err)
		}
	}
}

package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
)

func TestCopyDirFiltered_skipsNodeModulesAndAdminSrc(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	dst := t.TempDir()

	mustMkdirAll(t, filepath.Join(src, "admin", "ui-v2", "dist"))
	mustWrite(t, filepath.Join(src, "admin", "ui-v2", "dist", "index.html"), "ok")
	mustMkdirAll(t, filepath.Join(src, "admin", "ui-v2", "node_modules", "left-pad"))
	mustWrite(t, filepath.Join(src, "admin", "ui-v2", "node_modules", "left-pad", "index.js"), "x")
	mustMkdirAll(t, filepath.Join(src, "admin", "ui-v2", "src"))
	mustWrite(t, filepath.Join(src, "admin", "ui-v2", "src", "main.tsx"), "x")
	mustWrite(t, filepath.Join(src, "admin", "router.go"), "package admin")

	if err := copyDirFiltered(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "admin", "ui-v2", "dist", "index.html")); err != nil {
		t.Fatalf("expected dist/index.html vendored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "admin", "router.go")); err != nil {
		t.Fatalf("expected router.go vendored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "admin", "ui-v2", "node_modules")); !os.IsNotExist(err) {
		t.Fatal("node_modules should not be vendored")
	}
	if _, err := os.Stat(filepath.Join(dst, "admin", "ui-v2", "src")); !os.IsNotExist(err) {
		t.Fatal("ui-v2/src should not be vendored")
	}
}

func TestVerifyVendoredAdminDist_requiresIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := buildprofile.Profile{Capabilities: buildprofile.Capabilities{Admin: true}}
	if err := VerifyVendoredAdminDist(dir, prof); err == nil {
		t.Fatal("expected error when dist missing")
	}
	mustMkdirAll(t, filepath.Join(dir, "pkg", "admin", "ui-v2", "dist"))
	mustWrite(t, filepath.Join(dir, "pkg", "admin", "ui-v2", "dist", "index.html"), "ok")
	if err := VerifyVendoredAdminDist(dir, prof); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

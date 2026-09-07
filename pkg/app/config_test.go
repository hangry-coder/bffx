package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOverlay(t *testing.T) {
	tmp := t.TempDir()
	cfgDir := filepath.Join(tmp, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "port: 9090\nlogLevel: debug\nenv:\n  BFFX_TEST_OVERLAY: from_yaml\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "development.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BFFX_ENV", "development")
	t.Setenv("BFFX_TEST_OVERLAY", "")

	overlay, err := LoadConfigOverlay(tmp)
	if err != nil {
		t.Fatalf("LoadConfigOverlay: %v", err)
	}
	if overlay == nil {
		t.Fatal("expected overlay")
	}
	if overlay.Port != 9090 {
		t.Fatalf("port: got %d want 9090", overlay.Port)
	}
	if os.Getenv("BFFX_PORT") != "9090" {
		t.Fatalf("BFFX_PORT=%q want 9090", os.Getenv("BFFX_PORT"))
	}
	if os.Getenv("BFFX_TEST_OVERLAY") != "from_yaml" {
		t.Fatalf("env overlay: got %q", os.Getenv("BFFX_TEST_OVERLAY"))
	}
}

func TestLoadConfigOverlayMissingDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BFFX_ENV", "development")
	overlay, err := LoadConfigOverlay(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlay != nil {
		t.Fatal("expected nil overlay when config/ missing")
	}
}

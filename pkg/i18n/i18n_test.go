package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAndLoadOverride(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "assets", "i18n")
	featureDir := filepath.Join(tmp, "internal", "features", "auth", "i18n")
	for _, d := range []string{globalDir, featureDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(globalDir, "en.yaml"), []byte("greeting: Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "en.yaml"), []byte("greeting: Hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := NewBundle("en")
	if err := b.DiscoverAndLoad(tmp); err != nil {
		t.Fatalf("DiscoverAndLoad: %v", err)
	}
	if got := b.Translate("en", "greeting", nil); got != "Hi" {
		t.Fatalf("greeting: got %q want Hi (feature overrides global)", got)
	}
}

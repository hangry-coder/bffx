package generator

import (
	"strings"
	"testing"
)

func TestNormalizeStoreMode(t *testing.T) {
	got, err := normalizeStoreMode("")
	if err != nil || got != "sqlite" {
		t.Fatalf("empty: got %q err %v", got, err)
	}
	if _, err := normalizeStoreMode("bogus"); err == nil {
		t.Fatal("expected error for bogus store")
	}
}

func TestStoreSpecYAMLLinesPostgres(t *testing.T) {
	lines := storeSpecYAMLLines("postgres")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mode: postgres") {
		t.Fatalf("expected postgres mode: %s", joined)
	}
	if strings.Contains(joined, "app.db") {
		t.Fatalf("postgres spec should not include sqlite path")
	}
}

func TestDefaultEnvLinesForStorePostgres(t *testing.T) {
	lines := defaultEnvLinesForStore("postgres")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "DATABASE_URL=") {
		t.Fatalf("expected DATABASE_URL in env: %s", joined)
	}
}

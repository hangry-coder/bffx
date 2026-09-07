package dev

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestShouldRestart(t *testing.T) {
	if !ShouldRestart("hooks/foo.go", fsnotify.Write) {
		t.Fatal("expected .go write to restart")
	}
	if ShouldRestart("README.md", fsnotify.Write) {
		t.Fatal("did not expect .md write to restart")
	}
	if ShouldRestart("github.com/hangry-coder/bffx/x.yaml", fsnotify.Chmod) {
		t.Fatal("chmod should not restart")
	}
}

func TestDevWatchRoots_Legacy(t *testing.T) {
	tmp := t.TempDir()
	for _, d := range []string{"bffx", "hooks", "cmd/orchestrator"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roots := DevWatchRoots(tmp)
	if len(roots) < 2 {
		t.Fatalf("expected at least bffx and hooks, got %v", roots)
	}
}

func TestDevWatchRoots_V2(t *testing.T) {
	tmp := t.TempDir()
	for _, d := range []string{"bffx", "cmd/api", "internal/features"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roots := DevWatchRoots(tmp)
	if len(roots) < 2 {
		t.Fatalf("expected at least bffx and cmd/api, got %v", roots)
	}
}

package dev

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// DevWatchRoots returns directories to watch for bffx dev --watch.
func DevWatchRoots(projectRoot string) []string {
	root := filepath.Clean(projectRoot)
	var dirs []string
	candidates := []string{
		filepath.Join(root, "bffx"),
		filepath.Join(root, "internal", "features"),
		filepath.Join(root, "cmd", "orchestrator"),
		filepath.Join(root, "cmd", "api"),
	}
	// Legacy layout only: top-level hooks/ (v2 uses internal/features/*/hooks).
	if st, err := os.Stat(filepath.Join(root, "hooks")); err == nil && st.IsDir() {
		if entries, err := os.ReadDir(filepath.Join(root, "hooks")); err == nil {
			for _, e := range entries {
				if e.Name() != "doc.go" || len(entries) > 1 {
					candidates = append([]string{filepath.Join(root, "hooks")}, candidates...)
					break
				}
			}
		}
	}
	for _, d := range candidates {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// AddRecursiveWatch registers [w] for all directories under roots (skips dot dirs).
func AddRecursiveWatch(w *fsnotify.Watcher, roots []string) error {
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") {
					return filepath.SkipDir
				}
				return w.Add(path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ShouldRestart reports whether a filesystem event should trigger a dev rebuild.
func ShouldRestart(name string, op fsnotify.Op) bool {
	if op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

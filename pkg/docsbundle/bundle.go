// Package docsbundle copies a curated BFFX documentation set into a project for the admin panel guide (dev only).
package docsbundle

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPaths are framework docs copied into .bffx/docs/ on vendor / scaffold.
var DefaultPaths = []string{
	"README.md",
	"getting-started/quickstart.md",
	"getting-started.md",
	"reference/cli.md",
	"core-concepts/admin_manifest.md",
	"guides/auth.md",
	"guides/deployment/README.md",
	"guides/environment.md",
}

// IndexEntry is one guide document in the admin panel.
type IndexEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Index is written to .bffx/docs/index.json.
type Index struct {
	Entries []IndexEntry `json:"entries"`
}

// Bundle copies documentation from frameworkRoot/docs into projectRoot/.bffx/docs.
func Bundle(projectRoot, frameworkRoot string) error {
	srcDocs := filepath.Join(frameworkRoot, "docs")
	if _, err := os.Stat(srcDocs); err != nil {
		return fmt.Errorf("framework docs not found at %s: %w", srcDocs, err)
	}

	dstDir := filepath.Join(projectRoot, ".bffx", "docs")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	var entries []IndexEntry
	for _, rel := range DefaultPaths {
		src := filepath.Join(srcDocs, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		dst := filepath.Join(dstDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		title := titleFromFile(dst)
		id := strings.TrimSuffix(relSlash, filepath.Ext(relSlash))
		entries = append(entries, IndexEntry{ID: id, Title: title, Path: relSlash})
	}

	idx := Index{Entries: entries}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dstDir, "index.json"), data, 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func titleFromFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return filepath.Base(path)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// ProjectDocsDir returns the bundled docs directory for a project.
func ProjectDocsDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".bffx", "docs")
}

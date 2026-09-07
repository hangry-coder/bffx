package schema

import (
	"os"
	"path/filepath"
	"strings"
)

// MigrationsAdopted reports whether the project uses the versioned migration
// baseline (schema.json snapshot and/or at least one *.up.sql file).
// When false, declarative reconcile may still run, but schema.json drift diffs
// against an empty snapshot are not meaningful.
func MigrationsAdopted(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "schema.json")); err == nil {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".up.sql") {
			return true
		}
	}
	return false
}

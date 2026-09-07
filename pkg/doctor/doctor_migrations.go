package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

func lintVersionedMigrations(root string, reg *manifest.Registry) []Result {
	spec := reg.ProjectSpec()
	switch spec.Store.Mode {
	case "sqlite", "postgres":
	default:
		return nil
	}

	migrationsDir := filepath.Join(root, "migrations")
	if _, err := os.Stat(filepath.Join(root, "db", "migrations")); err == nil {
		migrationsDir = filepath.Join(root, "db", "migrations")
	}

	if schema.MigrationsAdopted(migrationsDir) {
		return []Result{{
			Name:    "Versioned migrations",
			Status:  "ok",
			Message: fmt.Sprintf("Migration baseline present under %s", migrationsDir),
		}}
	}

	return []Result{{
		Name:   "Versioned migrations",
		Status: "warn",
		Message: fmt.Sprintf(
			"No schema.json or *.up.sql under %s — sync skips schema drift diffs until you run `bffx migrate init` (after the database exists from first `bffx dev` or sqlite seed).",
			migrationsDir,
		),
	}}
}

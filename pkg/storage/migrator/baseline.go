package migrator

import (
	"database/sql"
	"fmt"

	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

// DatabaseHasAnyProjectTable returns true if at least one table from the migration
// snapshot exists in the database (used to avoid stamping migrate init on empty DBs).
func DatabaseHasAnyProjectTable(db *sql.DB, driver string, snap *schema.SchemaSnapshot) bool {
	if db == nil || snap == nil || len(snap.Tables) == 0 {
		return false
	}
	for table := range snap.Tables {
		if tableExists(db, driver, table) {
			return true
		}
	}
	return false
}

func tableExists(db *sql.DB, driver, table string) bool {
	var exists bool
	switch driver {
	case "postgres":
		err := db.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table,
		).Scan(&exists)
		return err == nil && exists
	case "sqlite", "sqlite3":
		err := db.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?
			)`, table,
		).Scan(&exists)
		return err == nil && exists
	default:
		return false
	}
}

// MissingTablesMessage suggests recovery when migrations are stamped but tables are absent.
func MissingTablesMessage(version uint) string {
	return fmt.Sprintf(
		"migration version %d is marked applied but project tables are missing — drop table schema_migrations, then run `bffx migrate apply` (avoid `migrate force 0`; it breaks golang-migrate)",
		version,
	)
}

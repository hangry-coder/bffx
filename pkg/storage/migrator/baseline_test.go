package migrator

import (
	"database/sql"
	"testing"

	"github.com/hangry-coder/bffx/pkg/storage/schema"
	_ "github.com/mattn/go-sqlite3"
)

func TestDatabaseHasAnyProjectTable_sqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	snap := &schema.SchemaSnapshot{
		Tables: map[string]schema.TableDef{
			"bffx_user": {Name: "bffx_user"},
		},
	}
	if DatabaseHasAnyProjectTable(db, "sqlite", snap) {
		t.Fatal("expected false on empty db")
	}

	if _, err := db.Exec(`CREATE TABLE bffx_user (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if !DatabaseHasAnyProjectTable(db, "sqlite", snap) {
		t.Fatal("expected true after create table")
	}
}

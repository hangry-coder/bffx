package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationsAdopted(t *testing.T) {
	dir := t.TempDir()
	if MigrationsAdopted(dir) {
		t.Fatal("empty dir should not be adopted")
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte(`{"tables":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !MigrationsAdopted(dir) {
		t.Fatal("schema.json should mark adopted")
	}
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "001_init.up.sql"), []byte("-- up"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !MigrationsAdopted(dir2) {
		t.Fatal("up.sql should mark adopted")
	}
}

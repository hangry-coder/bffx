package cli

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// handleDB implements the `bffx db ...` subcommand suite. Currently:
//
//   bffx db backup --path <file>  online VACUUM-INTO snapshot of the SQLite DB
//
// Other drivers (Postgres, Mongo, …) print a friendly note pointing operators
// to the right native tool (`pg_dump`, `mongodump`, etc.) — the framework does
// not try to ship custom backup routines for things that already have battle-
// tested external tooling.
func HandleDB(args []string) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println("Usage: bffx db [backup]")
			fmt.Println("  backup  SQLite online snapshot via VACUUM INTO (--path|-o FILE)")
			return
		}
	}
	if len(args) == 0 {
		fmt.Println("Usage: bffx db [backup]")
		return
	}

	switch args[0] {
	case "backup":
		var path string
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--path", "-o":
				if i+1 < len(args) {
					path = args[i+1]
					i++
				}
			}
		}
		if path == "" {
			path = filepath.Join("backups", fmt.Sprintf("bffx_%s.db", time.Now().Format("20060102_150405")))
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			log.Fatalf("create backup dir: %v", err)
		}

		root, _ := filepath.Abs(".")
		reg, err := manifest.LoadAll(root)
		if err != nil {
			log.Fatalf("failed to load registry: %v", err)
		}
		var projectSpec manifest.ProjectSpec
		if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
			log.Fatalf("failed to unmarshal project spec: %v", err)
		}
		store, err := storage.NewStore(root, &projectSpec, reg)
		if err != nil {
			log.Fatalf("failed to init store: %v", err)
		}

		var sqlDB *sql.DB
		unwrapped := storage.UnwrapStore(store)
		if router, ok := unwrapped.(*storage.RouterStore); ok {
			primary := storage.UnwrapStore(router.Primary)
			if sqlite, ok := primary.(*storage.SQLiteStore); ok {
				sqlDB = sqlite.GetDB()
			} else {
				log.Fatalf("`bffx db backup` currently supports SQLite only; for the configured driver, use the native backup tool (e.g. pg_dump for postgres)")
			}
		}
		if sqlDB == nil {
			log.Fatalf("could not obtain SQLite handle from configured store")
		}

		// VACUUM INTO produces a consistent snapshot of the live DB without
		// blocking writers. Quote the path to allow spaces.
		quoted := strings.ReplaceAll(path, `'`, `''`)
		if _, err := sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", quoted)); err != nil {
			log.Fatalf("vacuum into %q: %v", path, err)
		}
		fmt.Printf("✅ SQLite backup written to %s\n", path)

	default:
		fmt.Printf("Unknown db command: %s\n", args[0])
		fmt.Println("Usage: bffx db [backup]")
	}
}

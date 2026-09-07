package cli

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/storage/migrator"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

func HandleMigrate(args []string) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println("Usage: bffx migrate [init|plan|apply|status|diff|layout|packaging] [--root DIR]")
			fmt.Println("  Versioned SQL migrations for SQLite or Postgres (see docs/migration_guide.md).")
			fmt.Println("  packaging: guided full <-> minimal build profile migration.")
			return
		}
	}

	root := "."
	actArgs := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		} else {
			actArgs = append(actArgs, args[i])
		}
	}
	args = actArgs

	if len(args) == 0 {
		fmt.Println("Usage: bffx migrate [init|plan|apply|status|diff|layout|packaging] [--root DIR]")
		fmt.Println("  Versioned SQL migrations for SQLite or Postgres (see docs/migration_guide.md).")
		fmt.Println("  layout: print v2 layout migration dry-run (see plan/framework/v1/todo/bffx_comparison_v2.md).")
		fmt.Println("  packaging: guided full <-> minimal build profile migration.")
		return
	}

	if args[0] == "layout" {
		handleMigrateLayout(root, args[1:])
		return
	}

	if args[0] == "packaging" {
		handleMigratePackaging(root, args[1:])
		return
	}

	migrationsDir := filepath.Join(root, "migrations")

	// Load Registry
	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("failed to load registry: %v", err)
	}

	if reg.Project == nil {
		log.Fatalf("no project.yaml found in %s/bffx", root)
	}

	// Load Project Spec to get store info and layout
	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		log.Fatalf("failed to unmarshal project spec: %v", err)
	}

	if projectSpec.Layout == "v2" {
		migrationsDir = filepath.Join(root, "db", "migrations")
	} else if _, err := os.Stat(filepath.Join(root, "db", "migrations")); err == nil {
		migrationsDir = filepath.Join(root, "db", "migrations")
	}

	// Init Store to get SQL connection
	store, err := storage.NewStore(root, &projectSpec, reg)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	var sqlDB *sql.DB
	var driverName string

	// Unwrap RouterStore and other decorators if present
	unwrapped := storage.UnwrapStore(store)
	primary := unwrapped
	if router, ok := unwrapped.(*storage.RouterStore); ok {
		primary = storage.UnwrapStore(router.Primary)
	}

	if sqlite, ok := primary.(*storage.SQLiteStore); ok {
		sqlDB = sqlite.GetDB()
		driverName = "sqlite"
	} else if pg, ok := primary.(*storage.PostgresStore); ok {
		sqlDB = pg.GetDB()
		driverName = "postgres"
	}

	if sqlDB == nil && args[0] != "plan" && args[0] != "diff" {
		log.Fatalf("migrate %s requires a SQL database (SQLite/Postgres)", args[0])
	}

	switch args[0] {
	case "init":
		// Bootstrap an existing project onto the versioned migration system.
		// Generates an initial schema snapshot + initial up/down SQL based on the
		// current manifests, and (if a SQL DB is reachable) marks that baseline
		// as already-applied so existing tables aren't re-created.
		if _, err := os.Stat(migrationsDir); err == nil {
			entries, _ := os.ReadDir(migrationsDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
					log.Fatalf("migrations/ already initialized (found %s). Use `bffx migrate plan` for incremental changes.", e.Name())
				}
			}
		}

		desired := schema.FromRegistry(reg)
		ops := schema.Diff(desired, nil)
		if len(ops) == 0 {
			fmt.Println("No resources detected; nothing to baseline.")
			return
		}

		var dialect migrator.Dialect
		if driverName == "postgres" {
			dialect = &migrator.PostgresDialect{}
		} else {
			dialect = &migrator.SQLiteDialect{}
		}

		snap := &schema.SchemaSnapshot{Tables: make(map[string]schema.TableDef)}
		if err := migrator.GenerateMigration(ops, dialect, migrationsDir, "initial_schema", snap); err != nil {
			log.Fatalf("failed to generate baseline migration: %v", err)
		}

		if sqlDB != nil {
			if migrator.DatabaseHasAnyProjectTable(sqlDB, driverName, snap) {
				version := snap.Version
				if err := stampInitialVersion(sqlDB, driverName, version); err != nil {
					log.Printf("warning: baseline generated but could not stamp DB as applied: %v", err)
					log.Printf("If your DB already has these tables, run the SQL printed in docs/migration_guide.md to mark version %s as applied.", version)
				} else {
					fmt.Printf("Baselined existing schema as version %s (marked applied).\n", version)
				}
			} else {
				fmt.Println("Database has no project tables yet — run `bffx migrate apply` to create schema (do not skip on a fresh DB).")
			}
		} else {
			fmt.Println("Baseline migration generated. No SQL DB connection — skipped applied-stamp.")
		}
		fmt.Printf("Wrote initial migration to %s. Future changes: `bffx migrate plan <name>` then `bffx migrate apply`.\n", migrationsDir)

	case "plan":
		snap, _ := schema.LoadSnapshot(filepath.Join(migrationsDir, "schema.json"))
		desired := schema.FromRegistry(reg)
		ops := schema.Diff(desired, snap)
		if len(ops) == 0 {
			fmt.Println("No changes detected. Schema is up to date.")
			return
		}

		name := "migration"
		if len(args) > 1 {
			name = args[1]
		}

		var dialect migrator.Dialect
		if driverName == "postgres" {
			dialect = &migrator.PostgresDialect{}
		} else {
			dialect = &migrator.SQLiteDialect{}
		}

		if err := migrator.GenerateMigration(ops, dialect, migrationsDir, name, snap); err != nil {
			log.Fatalf("failed to generate migration: %v", err)
		}
		fmt.Printf("Generated migration in %s\n", migrationsDir)

	case "apply":
		if err := applyMigrations(sqlDB, driverName, migrationsDir); err != nil {
			log.Fatalf("apply failed: %v", err)
		}
		fmt.Println("Migrations applied successfully.")

	case "force":
		if len(args) < 2 {
			log.Fatalf("usage: bffx migrate force VERSION [--root DIR]")
		}
		version, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("invalid version: %v", err)
		}
		runner, err := migrator.NewRunner(sqlDB, driverName, migrationsDir)
		if err != nil {
			log.Fatalf("failed to init runner: %v", err)
		}
		defer runner.Close()

		if err := runner.Force(version); err != nil {
			log.Fatalf("force failed: %v", err)
		}
		fmt.Printf("Forced version to %d.\n", version)

	case "status":
		runner, err := migrator.NewRunner(sqlDB, driverName, migrationsDir)
		if err != nil {
			log.Fatalf("failed to init runner: %v", err)
		}
		defer runner.Close()

		v, dirty, err := runner.Status()
		if err != nil {
			log.Fatalf("status failed: %v", err)
		}
		fmt.Printf("Current Version: %d (Dirty: %v)\n", v, dirty)

	case "diff":
		snap, _ := schema.LoadSnapshot(filepath.Join(migrationsDir, "schema.json"))
		desired := schema.FromRegistry(reg)
		ops := schema.Diff(desired, snap)
		if len(ops) == 0 {
			fmt.Println("No changes detected.")
			return
		}
		for _, op := range ops {
			if op.Column != "" {
				fmt.Printf("%s: %s.%s\n", op.Kind, op.Table, op.Column)
			} else {
				fmt.Printf("%s: %s\n", op.Kind, op.Table)
			}
		}

	default:
		fmt.Printf("Unknown migrate command: %s\n", args[0])
	}
}

func applyMigrations(sqlDB *sql.DB, driverName, migrationsDir string) error {
	runner, err := migrator.NewRunner(sqlDB, driverName, migrationsDir)
	if err != nil {
		return fmt.Errorf("init runner: %w", err)
	}
	// Do not Close: golang-migrate closes the shared *sql.DB (breaks bffx seed/dev after apply).

	if err := runner.Apply(); err != nil {
		return err
	}

	snap, _ := schema.LoadSnapshot(filepath.Join(migrationsDir, "schema.json"))
	if !migrator.DatabaseHasAnyProjectTable(sqlDB, driverName, snap) {
		v, _, _ := runner.Status()
		if v > 0 {
			return fmt.Errorf("%s", migrator.MissingTablesMessage(v))
		}
	}
	return nil
}

// stampInitialVersion marks the supplied baseline version as already-applied
// in the golang-migrate tracking table, so the next `migrate apply` won't try
// to re-run the initial schema against an existing DB.
func migrationsDirForRoot(root string, projectSpec *manifest.ProjectSpec) string {
	migrationsDir := filepath.Join(root, "migrations")
	if projectSpec != nil && projectSpec.Layout == "v2" {
		migrationsDir = filepath.Join(root, "db", "migrations")
	} else if _, err := os.Stat(filepath.Join(root, "db", "migrations")); err == nil {
		migrationsDir = filepath.Join(root, "db", "migrations")
	}
	return migrationsDir
}

func resolveSQLDBFromStore(store storage.Store) (*sql.DB, string) {
	primary := storage.UnwrapStore(store)
	if router, ok := primary.(*storage.RouterStore); ok {
		primary = storage.UnwrapStore(router.Primary)
	}
	if sqlite, ok := primary.(*storage.SQLiteStore); ok {
		return sqlite.GetDB(), "sqlite"
	}
	if pg, ok := primary.(*storage.PostgresStore); ok {
		return pg.GetDB(), "postgres"
	}
	return nil, ""
}

func stampInitialVersion(db *sql.DB, driver, version string) error {
	if version == "" {
		return fmt.Errorf("empty version")
	}
	var createSQL string
	switch driver {
	case "sqlite":
		createSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			dirty INTEGER NOT NULL
		)`
	case "postgres":
		createSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			dirty BOOLEAN NOT NULL
		)`
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	v, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		return fmt.Errorf("parse version %q: %w", version, err)
	}
	switch driver {
	case "sqlite":
		_, err = db.Exec("INSERT OR REPLACE INTO schema_migrations(version, dirty) VALUES(?, 0)", v)
	case "postgres":
		_, err = db.Exec("INSERT INTO schema_migrations(version, dirty) VALUES($1, false) ON CONFLICT (version) DO NOTHING", v)
	}
	return err
}

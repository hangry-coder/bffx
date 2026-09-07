package migrator

import (
	"database/sql"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Runner wraps golang-migrate to handle schema migrations
type Runner struct {
	m *migrate.Migrate
}

// NewRunner creates a new migration runner for SQL databases
func NewRunner(db *sql.DB, driverName, migrationsDir string) (*Runner, error) {
	var driver database.Driver
	var err error

	// Normalize driver name
	if driverName == "sqlite" {
		driverName = "sqlite3"
	}

	switch driverName {
	case "sqlite3":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
	case "postgres":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
	default:
		return nil, fmt.Errorf("unsupported migration driver: %s", driverName)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		driverName,
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize migrator: %w", err)
	}

	return &Runner{m: m}, nil
}

// Status returns the applied and pending migrations (simplified)
func (r *Runner) Status() (version uint, dirty bool, err error) {
	v, d, err := r.m.Version()
	if err == migrate.ErrNilVersion {
		return 0, false, nil
	}
	return v, d, err
}

// Apply runs all pending UP migrations
func (r *Runner) Apply() error {
	err := r.m.Up()
	if err == migrate.ErrNoChange {
		return nil
	}
	return err
}

// Rollback undoes the last applied migration
func (r *Runner) Rollback() error {
	return r.m.Steps(-1)
}

// Force sets a specific migration version (marks as not dirty)
func (r *Runner) Force(version int) error {
	return r.m.Force(version)
}

// Close releases resources
func (r *Runner) Close() {
	r.m.Close()
}

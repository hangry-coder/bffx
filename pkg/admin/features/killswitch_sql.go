package features

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SQLKillSwitchStore persists KillSwitch rows in the `bffx_screen_kill_switch`
// table. It supports SQLite and Postgres; the driver is passed in to handle
// dialect differences (placeholders).
type SQLKillSwitchStore struct {
	db         *sql.DB
	driver     string // "sqlite" or "postgres"
	ensureOnce sync.Once
	ensureErr  error
}

// NewSQLKillSwitchStore wraps a *sql.DB and lazily creates the table on first
// access. driver must be "sqlite" or "postgres".
func NewSQLKillSwitchStore(db *sql.DB, driver string) *SQLKillSwitchStore {
	return &SQLKillSwitchStore{db: db, driver: strings.ToLower(driver)}
}

func (s *SQLKillSwitchStore) ensure(ctx context.Context) error {
	s.ensureOnce.Do(func() {
		ddl := s.tableDDL()
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			s.ensureErr = fmt.Errorf("create bffx_screen_kill_switch: %w", err)
		}
	})
	return s.ensureErr
}

func (s *SQLKillSwitchStore) tableDDL() string {
	switch s.driver {
	case "postgres":
		return `CREATE TABLE IF NOT EXISTS bffx_screen_kill_switch (
			screen      TEXT NOT NULL,
			section     TEXT NOT NULL DEFAULT '',
			enabled     BOOLEAN NOT NULL DEFAULT FALSE,
			reason      TEXT NOT NULL DEFAULT '',
			expires_at  TIMESTAMPTZ,
			updated_by  TEXT NOT NULL DEFAULT '',
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (screen, section)
		)`
	default: // sqlite
		return `CREATE TABLE IF NOT EXISTS bffx_screen_kill_switch (
			screen      TEXT NOT NULL,
			section     TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 0,
			reason      TEXT NOT NULL DEFAULT '',
			expires_at  DATETIME,
			updated_by  TEXT NOT NULL DEFAULT '',
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (screen, section)
		)`
	}
}

func (s *SQLKillSwitchStore) ph(i int) string {
	if s.driver == "postgres" {
		return "$" + fmt.Sprint(i)
	}
	return "?"
}

func (s *SQLKillSwitchStore) Set(ctx context.Context, screen, section string, upd KillSwitchUpdate) (KillSwitch, error) {
	if err := s.ensure(ctx); err != nil {
		return KillSwitch{}, err
	}
	now := time.Now().UTC()
	row := KillSwitch{
		Screen:    screen,
		Section:   section,
		Enabled:   upd.Enabled,
		Reason:    upd.Reason,
		ExpiresAt: upd.ExpiresAt,
		UpdatedBy: upd.UpdatedBy,
		UpdatedAt: now,
	}

	var q string
	switch s.driver {
	case "postgres":
		q = `INSERT INTO bffx_screen_kill_switch (screen, section, enabled, reason, expires_at, updated_by, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (screen, section) DO UPDATE SET
				enabled = EXCLUDED.enabled,
				reason  = EXCLUDED.reason,
				expires_at = EXCLUDED.expires_at,
				updated_by = EXCLUDED.updated_by,
				updated_at = EXCLUDED.updated_at`
	default:
		q = `INSERT INTO bffx_screen_kill_switch (screen, section, enabled, reason, expires_at, updated_by, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(screen, section) DO UPDATE SET
				enabled = excluded.enabled,
				reason  = excluded.reason,
				expires_at = excluded.expires_at,
				updated_by = excluded.updated_by,
				updated_at = excluded.updated_at`
	}

	_, err := s.db.ExecContext(ctx, q, row.Screen, row.Section, row.Enabled, row.Reason, row.ExpiresAt, row.UpdatedBy, row.UpdatedAt)
	if err != nil {
		return KillSwitch{}, fmt.Errorf("set kill switch: %w", err)
	}
	return row, nil
}

func (s *SQLKillSwitchStore) Get(ctx context.Context, screen, section string) (*KillSwitch, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	// #nosec G202
	q := `SELECT screen, section, enabled, reason, expires_at, updated_by, updated_at
		FROM bffx_screen_kill_switch
		WHERE screen = ` + s.ph(1) + ` AND section = ` + s.ph(2) + ` LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, screen, section)

	var ks KillSwitch
	var expires sql.NullTime
	var enabled any
	if err := row.Scan(&ks.Screen, &ks.Section, &enabled, &ks.Reason, &expires, &ks.UpdatedBy, &ks.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get kill switch: %w", err)
	}
	ks.Enabled = anyAsBool(enabled)
	if expires.Valid {
		t := expires.Time
		ks.ExpiresAt = &t
	}
	// Auto-revert expired switches at read time so the layered evaluator
	// treats them as no-op.
	if ks.ExpiresAt != nil && time.Now().UTC().After(*ks.ExpiresAt) {
		return nil, nil
	}
	return &ks, nil
}

func (s *SQLKillSwitchStore) Delete(ctx context.Context, screen, section string) error {
	if err := s.ensure(ctx); err != nil {
		return err
	}
	// #nosec G202
	q := `DELETE FROM bffx_screen_kill_switch WHERE screen = ` + s.ph(1) + ` AND section = ` + s.ph(2)
	_, err := s.db.ExecContext(ctx, q, screen, section)
	if err != nil {
		return fmt.Errorf("delete kill switch: %w", err)
	}
	return nil
}

func (s *SQLKillSwitchStore) List(ctx context.Context) ([]KillSwitch, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	q := `SELECT screen, section, enabled, reason, expires_at, updated_by, updated_at
		FROM bffx_screen_kill_switch
		ORDER BY screen, section`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list kill switches: %w", err)
	}
	defer rows.Close()

	out := make([]KillSwitch, 0, 8)
	for rows.Next() {
		var ks KillSwitch
		var expires sql.NullTime
		var enabled any
		if err := rows.Scan(&ks.Screen, &ks.Section, &enabled, &ks.Reason, &expires, &ks.UpdatedBy, &ks.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan kill switch row: %w", err)
		}
		ks.Enabled = anyAsBool(enabled)
		if expires.Valid {
			t := expires.Time
			ks.ExpiresAt = &t
		}
		out = append(out, ks)
	}
	return out, rows.Err()
}

func anyAsBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case []byte:
		s := strings.ToLower(string(t))
		return s == "true" || s == "1" || s == "t"
	case string:
		s := strings.ToLower(t)
		return s == "true" || s == "1" || s == "t"
	}
	return false
}

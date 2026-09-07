package features

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func newSQLiteStore(t *testing.T) *SQLKillSwitchStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLKillSwitchStore(db, "sqlite")
}

func TestSQLKillSwitchStore_SetGetDelete(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, Reason: "rollout", UpdatedBy: "alice@example.com"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get(ctx, "home", "")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || !got.Enabled || got.Reason != "rollout" || got.UpdatedBy != "alice@example.com" {
		t.Fatalf("unexpected row: %+v", got)
	}

	// Upsert path: Set again with same key should update, not duplicate.
	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: false, Reason: "rolled back"}); err != nil {
		t.Fatalf("Set (upsert): %v", err)
	}
	rows, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected upsert to produce 1 row, got %d", len(rows))
	}
	if rows[0].Enabled {
		t.Fatalf("expected Enabled=false after rollback upsert, got %+v", rows[0])
	}

	if err := store.Delete(ctx, "home", ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = store.Get(ctx, "home", "")
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestSQLKillSwitchStore_AutoRevert(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour).UTC()
	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, ExpiresAt: &past, Reason: "expired"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get(ctx, "home", "")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected auto-revert (nil), got %+v", got)
	}

	// List should still surface the historical row.
	rows, _ := store.List(ctx)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row in List, got %d", len(rows))
	}
}

func TestSQLKillSwitchStore_SectionAndScreenCoexist(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, Reason: "whole"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set(ctx, "home", "weekly", KillSwitchUpdate{Enabled: true, Reason: "section"}); err != nil {
		t.Fatal(err)
	}

	whole, _ := store.Get(ctx, "home", "")
	if whole == nil || whole.Reason != "whole" {
		t.Fatalf("whole-screen lookup: %+v", whole)
	}
	sec, _ := store.Get(ctx, "home", "weekly")
	if sec == nil || sec.Reason != "section" {
		t.Fatalf("section lookup: %+v", sec)
	}

	all, _ := store.List(ctx)
	if len(all) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(all))
	}
}

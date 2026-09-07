package features

import (
	"context"
	"testing"
	"time"
)

func TestKillSwitch_IsActive(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	cases := []struct {
		name string
		ks   *KillSwitch
		want bool
	}{
		{"nil", nil, false},
		{"disabled", &KillSwitch{Enabled: false}, false},
		{"enabled no expiry", &KillSwitch{Enabled: true}, true},
		{"enabled expires future", &KillSwitch{Enabled: true, ExpiresAt: &future}, true},
		{"enabled expired", &KillSwitch{Enabled: true, ExpiresAt: &past}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.ks.IsActive(now); got != c.want {
				t.Fatalf("IsActive=%v, want %v", got, c.want)
			}
		})
	}
}

func TestMemoryKillSwitchStore_SetGetDelete(t *testing.T) {
	store := NewMemoryKillSwitchStore()
	ctx := context.Background()

	ks, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, Reason: "deploy bug"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if ks.Screen != "home" || !ks.Enabled || ks.Reason != "deploy bug" {
		t.Fatalf("unexpected set result: %+v", ks)
	}

	got, err := store.Get(ctx, "home", "")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || !got.Enabled {
		t.Fatalf("expected to retrieve enabled switch, got %+v", got)
	}

	if err := store.Delete(ctx, "home", ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = store.Get(ctx, "home", "")
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestMemoryKillSwitchStore_AutoRevert(t *testing.T) {
	clk := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewMemoryKillSwitchStore()
	store.SetClock(func() time.Time { return clk })
	ctx := context.Background()

	expires := clk.Add(30 * time.Minute)
	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, ExpiresAt: &expires}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Within the window — switch is active.
	got, _ := store.Get(ctx, "home", "")
	if got == nil || !got.Enabled {
		t.Fatalf("expected active switch within expiry window, got %+v", got)
	}

	// Advance past expiry — Get returns nil even though the row still exists.
	clk = clk.Add(1 * time.Hour)
	got, _ = store.Get(ctx, "home", "")
	if got != nil {
		t.Fatalf("expected auto-reverted switch (nil), got %+v", got)
	}

	// List still surfaces the row.
	rows, _ := store.List(ctx)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row in List, got %d", len(rows))
	}
}

func TestEvaluate_SectionThenScreenPrecedence(t *testing.T) {
	store := NewMemoryKillSwitchStore()
	ctx := context.Background()

	// No switches set.
	if ks, _ := Evaluate(ctx, store, "home", ""); ks != nil {
		t.Fatalf("expected nil with no switches, got %+v", ks)
	}

	// Screen-level switch only — affects whole screen and any section query.
	if _, err := store.Set(ctx, "home", "", KillSwitchUpdate{Enabled: true, Reason: "incident-123"}); err != nil {
		t.Fatal(err)
	}
	if ks, _ := Evaluate(ctx, store, "home", ""); ks == nil || ks.Reason != "incident-123" {
		t.Fatalf("expected screen-level switch active, got %+v", ks)
	}
	if ks, _ := Evaluate(ctx, store, "home", "weekly"); ks == nil || ks.Reason != "incident-123" {
		t.Fatalf("expected screen-level switch to dominate section query, got %+v", ks)
	}

	// Section-level switch — when section is queried, section wins.
	store.Delete(ctx, "home", "")
	if _, err := store.Set(ctx, "home", "weekly", KillSwitchUpdate{Enabled: true, Reason: "section bug"}); err != nil {
		t.Fatal(err)
	}
	if ks, _ := Evaluate(ctx, store, "home", "weekly"); ks == nil || ks.Reason != "section bug" {
		t.Fatalf("expected section-level switch, got %+v", ks)
	}
	// But whole-screen query has no switch set.
	if ks, _ := Evaluate(ctx, store, "home", ""); ks != nil {
		t.Fatalf("expected nil when querying screen with only section switch, got %+v", ks)
	}
}

func TestEvaluate_NilStoreSafe(t *testing.T) {
	ks, err := Evaluate(context.Background(), nil, "x", "y")
	if err != nil || ks != nil {
		t.Fatalf("nil store should return (nil, nil), got (%+v, %v)", ks, err)
	}
}

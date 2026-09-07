package api_test

import (
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"os"
	"testing"
	"time"
)

func TestOwnershipFields(t *testing.T) {
	t.Run("memory store adds timestamps on create", func(t *testing.T) {
		s := storage.NewMemoryStore()
		record, err := s.Create(context.Background(), "Item", map[string]any{"name": "test item"})
		if err != nil {
			t.Fatalf("expected created record, got err: %v", err)
		}
		createdAt, ok := record["created_at"].(string)
		if !ok || createdAt == "" {
			t.Fatalf("created_at missing or invalid: %#v", record["created_at"])
		}
		if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
			t.Fatalf("created_at is not RFC3339: %v", err)
		}
		if updatedAt, ok := record["updated_at"].(string); !ok || updatedAt == "" {
			t.Fatalf("updated_at missing or invalid: %#v", record["updated_at"])
		}
	})

	t.Run("memory store updates updated_at only", func(t *testing.T) {
		s := storage.NewMemoryStore()
		record, _ := s.Create(context.Background(), "Item", map[string]any{"name": "test item"})
		originalUpdatedAt, _ := record["updated_at"].(string)
		originalCreatedAt, _ := record["created_at"].(string)

		time.Sleep(1100 * time.Millisecond)
		updated, err := s.Update(context.Background(), "Item", record["id"].(string), map[string]any{"name": "updated"})
		if err != nil {
			t.Fatal("expected update to succeed")
		}
		if updated["updated_at"] == originalUpdatedAt {
			t.Fatalf("updated_at did not change: %q", originalUpdatedAt)
		}
		if updated["created_at"] != originalCreatedAt {
			t.Fatalf("created_at changed unexpectedly: before=%q after=%q", originalCreatedAt, updated["created_at"])
		}
	})

	t.Run("sqlite store persists timestamps", func(t *testing.T) {
		dbPath := "test_ownership.db"
		defer os.Remove(dbPath)

		s, err := storage.NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatalf("failed to create sqlite store: %v", err)
		}
		defer s.GetDB().Close()

		if _, err := s.GetDB().Exec("CREATE TABLE IF NOT EXISTS note (id TEXT PRIMARY KEY, name TEXT, created_at TEXT, updated_at TEXT, created_by TEXT)"); err != nil {
			t.Fatalf("failed to create test table: %v", err)
		}

		record, err := s.Create(context.Background(), "Note", map[string]any{"name": "sqlite item"})
		if err != nil {
			t.Fatalf("expected created sqlite record, got err: %v", err)
		}
		id, _ := record["id"].(string)
		fetched, err := s.Get(context.Background(), "Note", id)
		if err != nil {
			t.Fatal("expected to fetch created sqlite record")
		}
		if fetched["created_at"] == nil || fetched["updated_at"] == nil {
			t.Fatalf("expected timestamps in fetched record: %#v", fetched)
		}
	})

	t.Run("owner policy evaluation", func(t *testing.T) {
		pe := auth.NewPolicyEngine()

		if !pe.Evaluate("owner", map[string]any{"sub": "user_123"}, map[string]any{"created_by": "user_123"}) {
			t.Fatal("expected owner policy to allow matching owner")
		}
		if pe.Evaluate("owner", map[string]any{"sub": "user_456"}, map[string]any{"created_by": "user_123"}) {
			t.Fatal("expected owner policy to deny non-owner")
		}
		if pe.Evaluate("owner", nil, map[string]any{"created_by": "user_123"}) {
			t.Fatal("expected owner policy to deny missing claims")
		}
	})
}

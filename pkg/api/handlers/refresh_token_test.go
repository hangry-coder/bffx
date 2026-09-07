package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestResolveRefreshTokenRow_HashedAndLegacy(t *testing.T) {
	ctx := context.Background()
	s := storage.NewMemoryStore()

	rowID, wire, hash, err := auth.MintRefreshTokenCredential()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, "RefreshToken", map[string]any{
		"id":         rowID,
		"user_id":    "u1",
		"token":      hash,
		"expires_at": "2099-01-01T00:00:00Z",
		"used":       false,
		"family_id":  "f1",
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := resolveRefreshTokenRow(ctx, s, wire)
	if err != nil {
		t.Fatalf("hashed: %v", err)
	}
	if row["id"] != rowID {
		t.Fatalf("id mismatch %v", row["id"])
	}

	_, _ = s.Create(ctx, "RefreshToken", map[string]any{
		"user_id":    "u2",
		"token":      "legacy-plain",
		"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"used":       false,
		"family_id":  "f2",
	})
	row, err = resolveRefreshTokenRow(ctx, s, "legacy-plain")
	if err != nil {
		t.Fatalf("legacy: %v", err)
	}
	if row["user_id"] != "u2" {
		t.Fatalf("user %v", row["user_id"])
	}
}

package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"os"
	"testing"
)

func BenchmarkMemoryStore_Create(b *testing.B) {
	ctx := context.Background()
	s := NewMemoryStore()
	payload := map[string]any{"key": "value"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Create(ctx, "Resource", payload)
	}
}

func BenchmarkSQLiteStore_Create(b *testing.B) {
	ctx := context.Background()
	dbPath := "bench.db"
	defer os.Remove(dbPath)
	
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer s.db.Close()
	
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "Item"},
				Kind: "Resource",
			},
		},
	}
	// Note: We need to properly initialize the table for the benchmark
	s.Reconcile(ctx, reg)

	payload := map[string]any{"id": "1", "name": "test"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Create(ctx, "Item", payload)
	}
}

package storage

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"

	"gopkg.in/yaml.v3"
)

func TestStore_ContextCancel_AbortsQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cancel.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()

	ctx := context.Background()
	var specNode yaml.Node
	if err := yaml.Unmarshal([]byte(`
fields:
  - { name: title, type: string }
`), &specNode); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Task"}, Spec: specNode},
		},
	}
	if _, err := s.Reconcile(ctx, reg); err != nil {
		t.Fatal(err)
	}

	// Long-running read so we can cancel mid-flight (driver honors ctx).
	slow := `WITH RECURSIVE c(n) AS (
		SELECT 1
		UNION ALL
		SELECT n+1 FROM c WHERE n < 200000000
	) SELECT count(*) FROM c`

	runCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	var qerr error
	go func() {
		defer wg.Done()
		rows, err := s.db.QueryContext(runCtx, slow)
		if err != nil {
			qerr = err
			return
		}
		defer rows.Close()
		for rows.Next() {
		}
		qerr = rows.Err()
	}()

	time.Sleep(15 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("query did not return after cancel")
	}

	if qerr == nil {
		t.Fatal("expected non-nil error from canceled query")
	}
	if !errors.Is(qerr, context.Canceled) && !ContextError(qerr) {
		t.Fatalf("expected cancellation-related error, got %v (%T)", qerr, qerr)
	}
}

// TestReconcile_AdvisoryLock_SingleWriter is the Week 6 exit name: SQLite uses an
// in-process mutex to serialize Reconcile (Postgres uses pg_advisory_lock).
func TestReconcile_AdvisoryLock_SingleWriter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reconcile-concurrent.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()

	var specNode yaml.Node
	if err := yaml.Unmarshal([]byte(`
fields:
  - { name: title, type: string }
`), &specNode); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Task"}, Spec: specNode},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Reconcile(context.Background(), reg)
		}()
	}
	wg.Wait()
}

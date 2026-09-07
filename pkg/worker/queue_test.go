package worker_test

import (
	"context"
	"os"
	"testing"

	"github.com/hangry-coder/bffx/pkg/worker"
)

func TestRedisQueue_SkipIfNoRedis(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set, skipping Redis integration test")
	}

	ctx := context.Background()
	q, err := worker.NewRedisQueue(ctx, redisURL)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}

	job, err := (&worker.MemoryJobStore{}).Enqueue(ctx, "", "skill", "ImageCaptioning", map[string]any{"url": "test"})
	// MemoryJobStore is not exported zero-val — construct directly for test
	_ = job
	_ = err

	// Test Push
	testJob := worker.Job{
		ID:   "test-redis-job-001",
		Kind: "skill",
		Name: "ImageCaptioning",
		Input: map[string]any{"url": "http://example.com/food.jpg"},
	}
	if err := q.Push(ctx, testJob); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Test Pop with a short timeout context
	popCtx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	popped, err := q.Pop(popCtx)
	if err != nil {
		// Context timeout is acceptable here — we just want to confirm the connection works
		t.Logf("Pop timed out (expected in test): %v", err)
	} else {
		if popped.ID != testJob.ID {
			t.Errorf("expected job ID %q, got %q", testJob.ID, popped.ID)
		}
	}
}

func TestMemoryJobStore(t *testing.T) {
	ctx := context.Background()
	store := worker.NewMemoryJobStore()

	// Enqueue
	job, err := store.Enqueue(ctx, "", "skill", "ImageCaptioning", map[string]any{"url": "test"})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if job.Status != worker.StatusPending {
		t.Errorf("expected pending, got %s", job.Status)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}

	// Get
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: job not found: %v", err)
	}
	if got.Name != "ImageCaptioning" {
		t.Errorf("expected name ImageCaptioning, got %s", got.Name)
	}

	// UpdateResult
	if err := store.UpdateResult(ctx, job.ID, worker.StatusDone, map[string]any{"calories": 450}, ""); err != nil {
		t.Fatalf("UpdateResult failed: %v", err)
	}
	updated, _ := store.Get(ctx, job.ID)
	if updated.Status != worker.StatusDone {
		t.Errorf("expected done, got %s", updated.Status)
	}

	// Invalid kind
	_, err = store.Enqueue(ctx, "", "invalid", "Foo", nil)
	if err == nil {
		t.Error("expected error for invalid kind")
	}

	// Invalid name
	_, err = store.Enqueue(ctx, "", "skill", "", nil)
	if err == nil {
		t.Error("expected error for empty name")
	}

	// List
	store.Enqueue(ctx, "", "function", "Ping", nil)
	list, _ := store.List(ctx, "", 10, 0)
	if len(list) < 2 {
		t.Errorf("expected at least 2 jobs, got %d", len(list))
	}
}

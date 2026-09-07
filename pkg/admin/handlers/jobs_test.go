package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/worker"
)

type mockQueue struct {
	pushed []worker.Job
}

func (mq *mockQueue) Push(ctx context.Context, job worker.Job) error {
	mq.pushed = append(mq.pushed, job)
	return nil
}

func (mq *mockQueue) Pop(ctx context.Context) (worker.Job, error) {
	return worker.Job{}, nil
}

func TestJobsHandler(t *testing.T) {
	store := worker.NewMemoryJobStore()
	queue := &mockQueue{}
	handler := handlers.NewJobsHandler(store, queue)

	// Enqueue a test job
	job, err := store.Enqueue(context.Background(), "user1", "skill", "test-job", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	t.Run("ListJobs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/jobs", nil)
		w := httptest.NewRecorder()
		handler.ListJobs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var jobs []worker.Job
		if err := json.Unmarshal(w.Body.Bytes(), &jobs); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(jobs) != 1 {
			t.Errorf("expected 1 job, got %d", len(jobs))
		}
	})

	t.Run("GetJob", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/jobs/"+job.ID, nil)
		req.SetPathValue("id", job.ID)
		w := httptest.NewRecorder()
		handler.GetJob(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var j worker.Job
		if err := json.Unmarshal(w.Body.Bytes(), &j); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if j.ID != job.ID {
			t.Errorf("expected job ID %q, got %q", job.ID, j.ID)
		}
	})

	t.Run("CancelJob success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/jobs/"+job.ID+"/cancel", nil)
		req.SetPathValue("id", job.ID)
		w := httptest.NewRecorder()
		handler.CancelJob(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify job state
		j, _ := store.Get(context.Background(), job.ID)
		if j.Status != worker.StatusFailed || j.Error != "cancelled" {
			t.Errorf("expected status 'failed' with error 'cancelled', got status=%q, error=%q", j.Status, j.Error)
		}
	})

	t.Run("RetryJob success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/jobs/"+job.ID+"/retry", nil)
		req.SetPathValue("id", job.ID)
		w := httptest.NewRecorder()
		handler.RetryJob(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify job state
		j, _ := store.Get(context.Background(), job.ID)
		if j.Status != worker.StatusPending || j.Error != "" {
			t.Errorf("expected status 'pending' with empty error, got status=%q, error=%q", j.Status, j.Error)
		}

		// Verify job was pushed back to queue
		if len(queue.pushed) != 1 || queue.pushed[0].ID != job.ID {
			t.Errorf("expected job to be pushed to queue, queue.pushed=%v", queue.pushed)
		}
	})
}

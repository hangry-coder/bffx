package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobStore is the interface for persisting and querying jobs.
type JobStore interface {
	Enqueue(ctx context.Context, userID, kind, name string, input map[string]any) (Job, error)
	Get(ctx context.Context, id string) (Job, error)
	UpdateResult(ctx context.Context, id, status string, result map[string]any, errMsg string) error
	List(ctx context.Context, userID string, limit, offset int) ([]Job, error)
	Prune(ctx context.Context, days int) (int64, error)
	RetryJob(ctx context.Context, id string) error
	CancelJob(ctx context.Context, id string) error
}

// MemoryJobStore is a thread-safe in-memory implementation of JobStore.
type MemoryJobStore struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

// NewMemoryJobStore creates a new MemoryJobStore.
func NewMemoryJobStore() *MemoryJobStore {
	return &MemoryJobStore{jobs: make(map[string]Job)}
}

// Enqueue creates a new job with StatusPending and stores it.
func (s *MemoryJobStore) Enqueue(ctx context.Context, userID, kind, name string, input map[string]any) (Job, error) {
	if kind != "skill" && kind != "function" && kind != "pipeline.ingestion" {
		return Job{}, fmt.Errorf("invalid job kind %q: must be 'skill', 'function', or 'pipeline.ingestion'", kind)
	}
	if name == "" {
		return Job{}, fmt.Errorf("job name is required")
	}

	now := time.Now().UTC()
	job := Job{
		ID:        uuid.New().String(),
		UserID:    userID,
		Kind:      kind,
		Name:      name,
		Input:     input,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()

	return job, nil
}

// Get retrieves a job by ID.
func (s *MemoryJobStore) Get(ctx context.Context, id string) (Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("job not found: %s", id)
	}
	return j, nil
}

// UpdateResult sets the job status, result, and error message.
func (s *MemoryJobStore) UpdateResult(ctx context.Context, id, status string, result map[string]any, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	j.Status = status
	j.Result = result
	j.Error = errMsg
	j.UpdatedAt = time.Now().UTC()
	s.jobs[id] = j
	return nil
}

// List returns a paginated slice of jobs for a user.
func (s *MemoryJobStore) List(ctx context.Context, userID string, limit, offset int) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []Job
	for _, j := range s.jobs {
		if userID == "" || j.UserID == userID {
			filtered = append(filtered, j)
		}
	}

	if offset >= len(filtered) {
		return []Job{}, nil
	}
	end := offset + limit
	if end > len(filtered) || limit <= 0 {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}

func (s *MemoryJobStore) Prune(ctx context.Context, days int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	var count int64
	for id, j := range s.jobs {
		if j.UpdatedAt.Before(cutoff) {
			delete(s.jobs, id)
			count++
		}
	}
	return count, nil
}

func (s *MemoryJobStore) RetryJob(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	j.Status = StatusPending
	j.Error = ""
	j.UpdatedAt = time.Now().UTC()
	s.jobs[id] = j
	return nil
}

func (s *MemoryJobStore) CancelJob(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	if j.Status == StatusDone || j.Status == StatusFailed {
		return fmt.Errorf("cannot cancel job in status: %s", j.Status)
	}
	j.Status = StatusFailed
	j.Error = "cancelled"
	j.UpdatedAt = time.Now().UTC()
	s.jobs[id] = j
	return nil
}

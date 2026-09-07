package worker

import "time"

// Status constants for a Job lifecycle.
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Job represents a single unit of async work dispatched to the Python worker.
type Job struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Kind      string         `json:"kind"`   // "skill" or "function"
	Name      string         `json:"name"`   // e.g. "ImageCaptioning"
	Input     map[string]any `json:"input"`  // arbitrary input payload
	Status    string         `json:"status"` // pending | running | done | failed
	Result    map[string]any `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

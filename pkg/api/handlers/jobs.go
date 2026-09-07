package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/worker"
	"encoding/json"
	"net/http"
	"time"
)

// JobsHandler handles async job dispatch and status queries.
type JobsHandler struct {
	store worker.JobStore
	queue worker.Queue // may be nil if Redis is not configured
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(store worker.JobStore, queue worker.Queue) *JobsHandler {
	return &JobsHandler{store: store, queue: queue}
}

// Dispatch handles POST /api/v1/jobs/dispatch
func (h *JobsHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind  string         `json:"kind"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid json", "bad_request")
		return
	}

	userID := middleware.GetUserID(r.Context())
	job, err := h.store.Enqueue(r.Context(), userID, body.Kind, body.Name, body.Input)
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, err.Error(), "dispatch_failed")
		return
	}

	// Push to Redis queue if available
	if h.queue != nil {
		if err := h.queue.Push(r.Context(), job); err != nil {
			// Mark the job as failed immediately if we can't enqueue it
			h.store.UpdateResult(r.Context(), job.ID, worker.StatusFailed, nil, "queue unavailable: "+err.Error())
			observability.WorkerJobsTotal.WithLabelValues(body.Name, "failed").Inc()
			errors.WriteError(w, http.StatusServiceUnavailable, "worker queue unavailable", "queue_error")
			return
		}
	}

	observability.WorkerJobsTotal.WithLabelValues(body.Name, "pending").Inc()
	errors.WriteJSON(w, http.StatusCreated, job)
}

// GetJob handles GET /api/v1/jobs/{id}
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.store.Get(r.Context(), id)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}

	// Ownership check
	userID := middleware.GetUserID(r.Context())
	if job.UserID != "" && job.UserID != userID {
		errors.Write(w, errors.ErrForbidden)
		return
	}

	errors.WriteJSON(w, http.StatusOK, job)
}

// WriteBack handles PATCH /api/v1/jobs/{id}/result (called by the Python worker)
func (h *JobsHandler) WriteBack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
		Error  string         `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid json", "bad_request")
		return
	}

	if body.Status != worker.StatusDone && body.Status != worker.StatusFailed {
		errors.WriteError(w, http.StatusBadRequest, "status must be 'done' or 'failed'", "invalid_status")
		return
	}

	job, err := h.store.Get(r.Context(), id)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}

	if err := h.store.UpdateResult(r.Context(), id, body.Status, body.Result, body.Error); err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}

	observability.WorkerJobsTotal.WithLabelValues(job.Name, body.Status).Inc()
	duration := time.Since(job.CreatedAt).Seconds()
	observability.WorkerJobDuration.WithLabelValues(job.Name).Observe(duration)

	errors.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	jobs, err := h.store.List(r.Context(), userID, 100, 0)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}
	errors.WriteJSON(w, http.StatusOK, map[string]any{"items": jobs})
}

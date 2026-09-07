package handlers

import (
	"encoding/json"
	"github.com/hangry-coder/bffx/pkg/worker"
	"net/http"
)

type JobsHandler struct {
	jobStore worker.JobStore
	queue    worker.Queue
}

func NewJobsHandler(js worker.JobStore, q worker.Queue) *JobsHandler {
	return &JobsHandler{jobStore: js, queue: q}
}

func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, _ := h.jobStore.List(r.Context(), "", 100, 0)
	if jobs == nil {
		jobs = []worker.Job{}
	}
	json.NewEncoder(w).Encode(jobs)
}

func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.jobStore.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(job)
}

func (h *JobsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.jobStore.RetryJob(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.queue != nil {
		job, err := h.jobStore.Get(r.Context(), id)
		if err == nil {
			_ = h.queue.Push(r.Context(), job)
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "retried"})
}

func (h *JobsHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.jobStore.CancelJob(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

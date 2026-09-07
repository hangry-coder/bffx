package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

type LiveOpsHandler struct {
	scheduler *liveops.Scheduler
	reg       *manifest.Registry
}

func NewLiveOpsHandler(scheduler *liveops.Scheduler, reg *manifest.Registry) *LiveOpsHandler {
	return &LiveOpsHandler{scheduler: scheduler, reg: reg}
}

func (h *LiveOpsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	events, err := h.scheduler.ListEvents()
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, events)
}

func (h *LiveOpsHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.WriteError(w, http.StatusServiceUnavailable, "LiveOps scheduler not configured")
		return
	}
	var ev liveops.LiveOpsEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	created, err := h.scheduler.CreateEvent(ev)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusCreated, created)
}

func (h *LiveOpsHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}
	name := r.PathValue("name")
	ev, err := h.scheduler.GetEvent(name)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}
	errors.WriteJSON(w, http.StatusOK, ev)
}

func (h *LiveOpsHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.WriteError(w, http.StatusServiceUnavailable, "LiveOps scheduler not configured")
		return
	}
	name := r.PathValue("name")
	var ev liveops.LiveOpsEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	updated, err := h.scheduler.UpdateEvent(name, ev)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, updated)
}

func (h *LiveOpsHandler) ToggleEvent(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.WriteError(w, http.StatusServiceUnavailable, "LiveOps scheduler not configured")
		return
	}
	name := r.PathValue("name")
	ev, err := h.scheduler.GetEvent(name)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}

	var update struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	ev.Enabled = update.Enabled
	updated, err := h.scheduler.UpdateEvent(name, *ev)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, updated)
}

func (h *LiveOpsHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		errors.WriteError(w, http.StatusServiceUnavailable, "LiveOps scheduler not configured")
		return
	}
	name := r.PathValue("name")
	if err := h.scheduler.DeleteEvent(name); err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type IncidentsHandler struct {
	store storage.Store
}

func NewIncidentsHandler(store storage.Store) *IncidentsHandler {
	return &IncidentsHandler{store: store}
}

// GetIncidents returns a list of unresolved incidents
func (h *IncidentsHandler) GetIncidents(w http.ResponseWriter, r *http.Request) {
	prov := observability.GetIncidentProvider(h.store)
	records, err := prov.List(r.Context())
	if err != nil || records == nil {
		records = []observability.Incident{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// ResolveIncident marks an incident as resolved by ID
func (h *IncidentsHandler) ResolveIncident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	actorID, _ := r.Context().Value("admin_id").(string)
	if actorID == "" {
		actorID = "admin@internal.io"
	}
	prov := observability.GetIncidentProvider(h.store)
	err := prov.Resolve(r.Context(), id, actorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

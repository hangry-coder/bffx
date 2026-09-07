package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type ProvidersHandler struct {
	store storage.Store
}

func NewProvidersHandler(store storage.Store) *ProvidersHandler {
	return &ProvidersHandler{store: store}
}

func (h *ProvidersHandler) GetProviders(w http.ResponseWriter, r *http.Request) {
	info := observability.GetProvidersInfo(r.Context(), h.store)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

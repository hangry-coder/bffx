package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type SettingsHandler struct {
	store storage.Store
	reg   *manifest.Registry
}

func NewSettingsHandler(store storage.Store, reg *manifest.Registry) *SettingsHandler {
	return &SettingsHandler{store: store, reg: reg}
}

func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.List(r.Context(), "AppConfig", 1000, 0)
	if err != nil {
		records = []map[string]any{}
	}

	res := make(map[string]string)
	for _, rec := range records {
		key, _ := rec["config_key"].(string)
		val, _ := rec["config_value"].(string)
		if key != "" {
			res[key] = val
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	for key, val := range payload {
		existing, err := h.store.Query(ctx, "AppConfig").Where("config_key", "=", key).Execute(ctx)
		if err == nil && len(existing) > 0 {
			id, _ := existing[0]["id"].(string)
			_, _ = h.store.Update(ctx, "AppConfig", id, map[string]any{
				"config_value": val,
			})
		} else {
			_, _ = h.store.Create(ctx, "AppConfig", map[string]any{
				"config_key":   key,
				"config_value": val,
			})
		}
	}

	w.WriteHeader(http.StatusOK)
}

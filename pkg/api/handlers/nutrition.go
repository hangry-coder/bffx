package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"encoding/json"
	"net/http"
)

type NutritionHandler struct {
	provider nutrition.Provider
}

func NewNutritionHandler(provider nutrition.Provider) *NutritionHandler {
	return &NutritionHandler{provider: provider}
}

type NutritionRequest struct {
	Query string `json:"query"`
}

func (h *NutritionHandler) GetNutrition(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil || h.provider.Type() == "noop" {
		errors.WriteError(w, http.StatusNotImplemented, "Nutrition battery is not configured", "nutrition_not_configured")
		return
	}

	var req NutritionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		errors.WriteError(w, http.StatusBadRequest, "query is required")
		return
	}

	result, err := h.provider.FetchNutrition(r.Context(), req.Query)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, "Nutrition lookup failed: "+err.Error(), "nutrition_lookup_failed")
		return
	}

	errors.WriteJSON(w, http.StatusOK, result)
}

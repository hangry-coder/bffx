package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"encoding/base64"
	"encoding/json"
	"net/http"
)

type VLMHandler struct {
	provider vlm.Provider
}

func NewVLMHandler(provider vlm.Provider) *VLMHandler {
	return &VLMHandler{provider: provider}
}

type AnalyzeRequest struct {
	Prompt string `json:"prompt"`
	Image  string `json:"image"` // Base64 encoded image
}

func (h *VLMHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil || h.provider.Type() == "noop" {
		errors.WriteError(w, http.StatusNotImplemented, "VLM battery is not configured", "vlm_not_configured")
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		errors.WriteError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	var image []byte
	if req.Image != "" {
		var err error
		image, err = base64.StdEncoding.DecodeString(req.Image)
		if err != nil {
			errors.WriteError(w, http.StatusBadRequest, "invalid image base64")
			return
		}
	}

	result, err := h.provider.Analyze(r.Context(), image, req.Prompt)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, "VLM analysis failed: "+err.Error(), "vlm_analysis_failed")
		return
	}

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"result": result,
	})
}

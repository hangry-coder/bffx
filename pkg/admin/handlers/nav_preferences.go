package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hangry-coder/bffx/pkg/admin/navprefs"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

type NavPreferencesHandler struct {
	reg *manifest.Registry
}

func NewNavPreferencesHandler(reg *manifest.Registry) *NavPreferencesHandler {
	return &NavPreferencesHandler{reg: reg}
}

func (h *NavPreferencesHandler) Get(w http.ResponseWriter, r *http.Request) {
	p := navprefs.Load(h.reg.Root)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (h *NavPreferencesHandler) Put(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
		Pin  bool   `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	p := navprefs.Load(h.reg.Root)
	switch body.Kind {
	case "resource":
		p.Resources[body.Name] = body.Pin
	case "feature":
		p.Features[body.Name] = body.Pin
	default:
		http.Error(w, "kind must be resource or feature", http.StatusBadRequest)
		return
	}
	if err := navprefs.Save(h.reg.Root, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

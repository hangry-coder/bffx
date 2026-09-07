package handlers

import (
	"encoding/json"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"net/http"
)

type FlagHandlerV2 struct {
	provider featureflags.FlagProvider
	reg      *manifest.Registry
}

func NewFlagHandlerV2(provider featureflags.FlagProvider, reg *manifest.Registry) *FlagHandlerV2 {
	return &FlagHandlerV2{provider: provider, reg: reg}
}

func (h *FlagHandlerV2) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.provider.ListFlags()
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, flags)
}

func (h *FlagHandlerV2) CreateFlag(w http.ResponseWriter, r *http.Request) {
	var flag featureflags.FlagDefinition
	if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	created, err := h.provider.CreateFlag(flag)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusCreated, created)
}

func (h *FlagHandlerV2) GetFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	flag, err := h.provider.GetFlag(key)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}
	errors.WriteJSON(w, http.StatusOK, flag)
}

func (h *FlagHandlerV2) UpdateFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var flag featureflags.FlagDefinition
	if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	updated, err := h.provider.UpdateFlag(key, flag)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, updated)
}

func (h *FlagHandlerV2) ToggleFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	flag, err := h.provider.GetFlag(key)
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

	flag.Enabled = update.Enabled
	updated, err := h.provider.UpdateFlag(key, *flag)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, updated)
}

func (h *FlagHandlerV2) DeleteFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := h.provider.DeleteFlag(key); err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FlagHandlerV2) AddBinding(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var binding featureflags.FlagBinding
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	flag, err := h.provider.GetFlag(key)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}

	flag.Bindings = append(flag.Bindings, binding)
	updated, err := h.provider.UpdateFlag(key, *flag)
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	errors.WriteJSON(w, http.StatusOK, updated)
}

func (h *FlagHandlerV2) ListScreenSections(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	for _, m := range h.reg.Screens {
		var spec manifest.ScreenSpec
		if err := m.UnmarshalSpec(&spec); err == nil && (spec.Name == name || m.Metadata.Name == name) {
			errors.WriteJSON(w, http.StatusOK, spec.Sections)
			return
		}
	}
	errors.Write(w, errors.ErrNotFound)
}

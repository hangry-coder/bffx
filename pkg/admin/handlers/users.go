package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type UserHandler struct {
	store storage.Store
	reg   *manifest.Registry
}

func NewUserHandler(store storage.Store, reg *manifest.Registry) *UserHandler {
	return &UserHandler{store: store, reg: reg}
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	records, err := h.store.List(r.Context(), "User", limit, offset)
	if err != nil {
		records = []map[string]any{}
	}

	// Filter out sensitive fields
	sanitized := make([]map[string]any, 0, len(records))
	for _, record := range records {
		rec := make(map[string]any)
		for k, v := range record {
			if k == "password" || k == "otp_code" || k == "password_hash" {
				continue
			}
			rec[k] = v
		}
		sanitized = append(sanitized, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitized)
}

func (h *UserHandler) GetUserDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	userRecord, err := h.store.Get(r.Context(), "User", id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Filter out sensitive fields
	userClean := make(map[string]any)
	for k, v := range userRecord {
		if k == "password" || k == "otp_code" || k == "password_hash" {
			continue
		}
		userClean[k] = v
	}

	// Fetch associated devices if the resource exists
	var devices []map[string]any
	if _, ok := h.reg.GetResource("Device"); ok {
		devices, _ = h.store.Query(r.Context(), "Device").Where("user_id", "=", id).Execute(r.Context())
	}
	if devices == nil {
		devices = []map[string]any{}
	}

	// Fetch associated roles/tokens if the resource exists
	var userRoles []map[string]any
	if _, ok := h.reg.GetResource("UserRole"); ok {
		userRoles, _ = h.store.Query(r.Context(), "UserRole").Where("user_id", "=", id).Execute(r.Context())
	}
	if userRoles == nil {
		userRoles = []map[string]any{}
	}

	response := map[string]any{
		"user":    userClean,
		"devices": devices,
		"roles":   userRoles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

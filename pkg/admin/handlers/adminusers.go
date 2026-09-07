package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type AdminUserHandler struct {
	store   storage.Store
	auditor *audit.Auditor
}

func NewAdminUserHandler(store storage.Store, auditor *audit.Auditor) *AdminUserHandler {
	return &AdminUserHandler{
		store:   store,
		auditor: auditor,
	}
}

func (h *AdminUserHandler) DeleteAdminUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	// 1. Fetch the user being deleted to check their role
	user, err := h.store.Get(ctx, "AdminUser", id)
	if err != nil || user == nil {
		http.Error(w, "admin user not found", http.StatusNotFound)
		return
	}

	role, _ := user["role"].(string)
	if role == "Superuser" {
		// Count superusers
		records, err := h.store.Query(ctx, "AdminUser").Where("role", "=", "Superuser").Execute(ctx)
		if err == nil && len(records) <= 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Cannot delete the last superuser",
			})
			return
		}
	}

	// 2. Perform deletion
	if err := h.store.Delete(ctx, "AdminUser", id); err != nil {
		http.Error(w, "failed to delete admin user", http.StatusInternalServerError)
		return
	}

	// 3. Audit Log
	if h.auditor != nil {
		actorID, _ := ctx.Value("admin_id").(string)
		h.auditor.Record(ctx, audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "delete",
			ResourceKind: "AdminUser",
			ResourceID:   id,
			IPAddress:    r.RemoteAddr,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

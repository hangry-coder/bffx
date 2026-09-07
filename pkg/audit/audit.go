package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// AuditLog is the legacy admin-panel audit record shape stored in AuditLog resources.
// Runtime paths should prefer observability.AuditEntry via batteries/audit.Provider.
type AuditLog struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id"`
	ActorType    string    `json:"actor_type"`
	Action       string    `json:"action"`
	ResourceKind string    `json:"resource_kind"`
	ResourceID   string    `json:"resource_id"`
	Payload      string    `json:"payload"`
	IPAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}

// ToEntry converts a legacy admin audit record to the canonical runtime audit entry contract.
func (l AuditLog) ToEntry() observability.AuditEntry {
	return observability.EntryFromAdminAudit(
		l.ActorID,
		l.ActorType,
		l.Action,
		l.ResourceKind,
		l.ResourceID,
		l.Payload,
		l.IPAddress,
	)
}

type Auditor struct {
	store storage.Store
}

func NewAuditor(store storage.Store) *Auditor {
	return &Auditor{store: store}
}

func (a *Auditor) Record(ctx context.Context, log AuditLog) {
	log.CreatedAt = time.Now()
	data, _ := json.Marshal(log)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	// Ensure we have a table for AuditLog
	_, _ = a.store.Create(ctx, "AuditLog", m)
}

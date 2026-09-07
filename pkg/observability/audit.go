package observability

import (
	"time"

	auditbattery "github.com/hangry-coder/bffx/pkg/batteries/audit"
)

// Canonical audit contract types (core semantics layer).
type AuditLevel = auditbattery.Level
type AuditEntry = auditbattery.Entry
type AuditProvider = auditbattery.Provider

const (
	AuditLevelInfo     = auditbattery.LevelInfo
	AuditLevelWarning  = auditbattery.LevelWarning
	AuditLevelCritical = auditbattery.LevelCritical
)

// EntryFromAdminAudit converts legacy admin-panel audit records to the canonical entry shape.
func EntryFromAdminAudit(actorID, actorType, action, resourceKind, resourceID, payload, ip string) AuditEntry {
	metadata := map[string]any{}
	if actorType != "" {
		metadata["actor_type"] = actorType
	}
	if payload != "" {
		metadata["payload"] = payload
	}
	return AuditEntry{
		Timestamp:  time.Now(),
		UserID:     actorID,
		Action:     action,
		Resource:   resourceKind,
		ResourceID: resourceID,
		Metadata:   metadata,
		Level:      AuditLevelInfo,
		IP:         ip,
	}
}

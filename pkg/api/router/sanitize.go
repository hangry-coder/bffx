package router

import (
	"strings"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// SensitiveFieldDenylist represents keys that must never be returned in public APIs.
var SensitiveFieldDenylist = map[string]bool{
	"password":      true,
	"password_hash": true,
	"otp_code":      true,
	"otp":           true,
	"secret":        true,
	"token":         true,
	"refresh_token": true,
}

// SanitizeResourceResponse returns a sanitized version of the resource row.
func SanitizeResourceResponse(resourceName string, row map[string]any) map[string]any {
	if row == nil {
		return nil
	}

	normName := strings.ReplaceAll(strings.ToLower(resourceName), "_", "")
	if normName == "user" || normName == "bffxuser" {
		return handlers.SanitizeUser(row)
	}

	out := make(map[string]any, len(row))
	for k, v := range row {
		kLower := strings.ToLower(k)
		if SensitiveFieldDenylist[kLower] {
			continue
		}
		out[k] = v
	}
	return out
}

// SanitizeResourceListResponse filters out sensitive fields from a slice of row maps.
func SanitizeResourceListResponse(resourceName string, rows []map[string]any) []map[string]any {
	if rows == nil {
		return nil
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = SanitizeResourceResponse(resourceName, row)
	}
	return out
}

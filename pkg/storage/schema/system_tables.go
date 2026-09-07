package schema

import (
	"strings"
)

// P0 core resource names to their new prefixed physical table names
var systemTables = map[string]string{
	"user":         "bffx_user",
	"device":       "bffx_device",
	"adminuser":    "bffx_admin_user",
	"role":         "bffx_role",
	"permission":   "bffx_permission",
	"userrole":     "bffx_user_role",
	"refreshtoken": "bffx_refresh_token",
	"feature_flag": "bffx_feature_flag",
	"appconfig":    "bffx_app_config",
	"incident":     "bffx_incident",
	"telemetryevent": "bffx_telemetry_event",
	"telemetry_event": "bffx_telemetry_event",
	"metricsample":  "bffx_metric_sample",
	"metric_sample":  "bffx_metric_sample",
}

// ResolveTable maps a logical resource name to its physical database table name.
func ResolveTable(resourceName string) string {
	lowerName := strings.ToLower(resourceName)
	// Check P0 map
	if prefixed, exists := systemTables[lowerName]; exists {
		return prefixed
	}
	// Fallback to default
	return lowerName
}

// IsSystemResource returns true if the logical resource name is a system resource.
func IsSystemResource(resourceName string) bool {
	lowerName := strings.ToLower(resourceName)
	_, exists := systemTables[lowerName]
	return exists
}

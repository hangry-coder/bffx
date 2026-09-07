package handlers

import (
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func isSensitiveField(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "password") ||
		strings.Contains(n, "otp") ||
		strings.Contains(n, "secret") ||
		strings.Contains(n, "token") ||
		strings.Contains(n, "hash")
}

func sanitizeRecord(record map[string]any, resourceSpec *manifest.AdminResourceSpec) map[string]any {
	if record == nil {
		return nil
	}
	sanitized := make(map[string]any)
	for k, v := range record {
		if isSensitiveField(k) {
			continue
		}
		if resourceSpec != nil {
			excluded := false
			for _, excl := range resourceSpec.Show.Exclude {
				if strings.EqualFold(excl, k) {
					excluded = true
					break
				}
			}
			for _, excl := range resourceSpec.Form.Exclude {
				if strings.EqualFold(excl, k) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}
		sanitized[k] = v
	}
	return sanitized
}

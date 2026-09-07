package manifest

import (
	"strings"

	"gopkg.in/yaml.v3"
)

const builtinRefreshTokenYAML = `apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: RefreshToken
spec:
  routes:
    crud: false
  policy:
    read: owner
    write: owner
  fields:
    - {name: user_id, type: string, required: true}
    - {name: device_id, type: string}
    - {name: token, type: string}
    - {name: expires_at, type: string}
    - {name: used, type: bool}
    - {name: family_id, type: string}
    - {name: created_at, type: string}
    - {name: updated_at, type: string}
`

// ProjectRefreshTokensEnabled reports whether the project wants refresh-token storage.
// Default is true when unset (nil), so new apps get /auth/refresh without extra YAML.
func ProjectRefreshTokensEnabled(reg *Registry) bool {
	if reg.Project == nil {
		return true
	}
	var ps ProjectSpec
	if err := reg.Project.UnmarshalSpec(&ps); err != nil {
		return true
	}
	if ps.FeatureFlags.RefreshTokens != nil && !*ps.FeatureFlags.RefreshTokens {
		return false
	}
	return true
}

func injectBuiltinRefreshTokenResource(reg *Registry) {
	if !ProjectRefreshTokensEnabled(reg) {
		return
	}
	for _, r := range reg.Resources {
		if strings.EqualFold(strings.TrimSpace(r.Metadata.Name), "RefreshToken") {
			return
		}
	}
	var m Manifest
	if err := yaml.Unmarshal([]byte(builtinRefreshTokenYAML), &m); err != nil {
		reg.Warnings = append(reg.Warnings, "builtin RefreshToken manifest: "+err.Error())
		return
	}
	m.Path = ":builtin:RefreshToken"
	m.Raw = []byte(builtinRefreshTokenYAML)
	reg.Resources = append(reg.Resources, &m)
}

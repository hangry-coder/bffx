package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func getModulePath(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func getFeatureOfManifest(m *manifest.Manifest) string {
	if m == nil {
		return "app"
	}
	if m.Feature != "" {
		return strings.ToLower(m.Feature)
	}
	var spec struct {
		Group string `yaml:"group"`
	}
	_ = m.UnmarshalSpec(&spec)
	if spec.Group != "" {
		return strings.ToLower(spec.Group)
	}
	return "app"
}
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func toGRPCName(verb string, name string) string {
	if len(name) == 0 {
		return verb
	}
	camel := toCamelCase(name)
	if name[0] >= 'a' && name[0] <= 'z' {
		camel = strings.ToLower(camel[:1]) + camel[1:]
	}
	return verb + camel
}

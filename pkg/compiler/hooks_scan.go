package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

const (
	bffxHookTag     = "@bffx:hook"
	bffxActionTag   = "@bffx:action" // pipeline before/after hooks (legacy alias)
	bffxSkipHookTag = "@bffx:skip-hook"
)

// hookFuncDecl is a top-level function discovered in a hooks package.
type hookFuncDecl struct {
	Name      string
	Feature   string // v2 feature slice name; empty for legacy layout
	Comments  string
	Signature string
}

// hookScanResult indexes discovered hook functions for registry emission.
type hookScanResult struct {
	ByName      map[string]hookFuncDecl
	Lifecycle   map[string]bool
	Payload     map[string]bool
	FuncFeature map[string]string
}

// isRouterHookFuncSignature reports whether a func line matches router.HookFunc:
//
//	func Name(ctx *handlers.ActionContext, <any> map[string]any) error
func isRouterHookFuncSignature(funcLine string) bool {
	line := strings.TrimSpace(funcLine)
	if !strings.HasPrefix(line, "func ") {
		return false
	}
	parts := strings.Fields(line)
	if len(parts) < 2 || strings.HasPrefix(parts[1], "(") {
		return false
	}
	if strings.Contains(line, "http.ResponseWriter") {
		return false
	}
	if strings.Contains(line, "storage.Store") {
		return false
	}
	if strings.Contains(line, "context.Context") && !strings.Contains(line, "*handlers.ActionContext") {
		return false
	}
	if !strings.Contains(line, "*handlers.ActionContext") {
		return false
	}
	if !strings.Contains(line, "map[string]any") {
		return false
	}
	return strings.Contains(line, ") error")
}

func parseFuncName(funcLine string) string {
	line := strings.TrimSpace(funcLine)
	if !strings.HasPrefix(line, "func ") {
		return ""
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}
	name := parts[1]
	if idx := strings.Index(name, "("); idx != -1 {
		name = name[:idx]
	}
	if strings.HasPrefix(name, "(") {
		return ""
	}
	return name
}

func isLifecycleHookName(name string) bool {
	switch {
	case strings.HasPrefix(name, "BeforeCreate"),
		strings.HasPrefix(name, "AfterCreate"),
		strings.HasPrefix(name, "BeforeUpdate"),
		strings.HasPrefix(name, "AfterUpdate"),
		strings.HasPrefix(name, "BeforeDelete"),
		strings.HasPrefix(name, "AfterDelete"):
		return true
	default:
		return false
	}
}

func commentHasTag(comments, tag string) bool {
	return strings.Contains(comments, tag)
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	r := rune(name[0])
	return r >= 'A' && r <= 'Z'
}

func shouldRegisterPayloadHook(decl hookFuncDecl) bool {
	if !isExportedIdent(decl.Name) {
		return false
	}
	if commentHasTag(decl.Comments, bffxSkipHookTag) {
		return false
	}
	if commentHasTag(decl.Comments, bffxHookTag) || commentHasTag(decl.Comments, bffxActionTag) {
		return isRouterHookFuncSignature(decl.Signature)
	}
	return isRouterHookFuncSignature(decl.Signature)
}

func parseHookDeclsFromSource(content string) []hookFuncDecl {
	var decls []hookFuncDecl
	var pendingComments []string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			pendingComments = append(pendingComments, trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "func ") {
			name := parseFuncName(trimmed)
			if name != "" {
				decls = append(decls, hookFuncDecl{
					Name:      name,
					Comments:  strings.Join(pendingComments, "\n"),
					Signature: trimmed,
				})
			}
			pendingComments = nil
			continue
		}
		pendingComments = nil
	}
	return decls
}

func scanHookPackages(root string, isV2 bool) hookScanResult {
	result := hookScanResult{
		ByName:      make(map[string]hookFuncDecl),
		Lifecycle:   make(map[string]bool),
		Payload:     make(map[string]bool),
		FuncFeature: make(map[string]string),
	}

	walkHooksDir := func(hooksDir, featureName string) {
		_ = filepath.Walk(hooksDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			base := filepath.Base(path)
			if base == "doc.go" || strings.HasSuffix(base, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, decl := range parseHookDeclsFromSource(string(content)) {
				decl.Feature = featureName
				result.ByName[decl.Name] = decl
				result.FuncFeature[decl.Name] = featureName
				if isExportedIdent(decl.Name) && isLifecycleHookName(decl.Name) {
					result.Lifecycle[decl.Name] = true
				}
				if shouldRegisterPayloadHook(decl) {
					result.Payload[decl.Name] = true
				}
			}
			return nil
		})
	}

	if isV2 {
		featuresDir := filepath.Join(root, "internal", "features")
		if entries, err := os.ReadDir(featuresDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				featureName := strings.ToLower(entry.Name())
				hooksSubdir := filepath.Join(featuresDir, entry.Name(), "hooks")
				if _, err := os.Stat(hooksSubdir); err != nil {
					continue
				}
				walkHooksDir(hooksSubdir, featureName)
			}
		}
		return result
	}

	hooksDir := filepath.Join(root, "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		walkHooksDir(hooksDir, "")
	}
	return result
}

func collectManifestHookActions(reg *manifest.Registry) map[string]bool {
	refs := make(map[string]bool)
	add := func(action string) {
		action = strings.TrimSpace(action)
		if action != "" {
			refs[action] = true
		}
	}
	for _, m := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}
		for _, h := range spec.Hooks.BeforeCreate {
			add(h.Action)
		}
		for _, h := range spec.Hooks.AfterCreate {
			add(h.Action)
		}
		for _, h := range spec.Hooks.BeforeUpdate {
			add(h.Action)
		}
		for _, h := range spec.Hooks.AfterUpdate {
			add(h.Action)
		}
		for _, h := range spec.Hooks.BeforeDelete {
			add(h.Action)
		}
		for _, h := range spec.Hooks.AfterDelete {
			add(h.Action)
		}
	}
	for _, m := range reg.Pipelines {
		var spec manifest.PipelineSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}
		for _, h := range spec.Hooks.BeforePipeline {
			add(h.Action)
		}
		for _, h := range spec.Hooks.AfterPipeline {
			add(h.Action)
		}
	}
	return refs
}

func warnUnresolvedManifestHooks(registered map[string]bool, referenced map[string]bool) {
	for name := range referenced {
		if registered[name] {
			continue
		}
		logger.Warn("manifest references hook %q but no matching handler was registered — implement func %s(ctx *handlers.ActionContext, payload map[string]any) error in a hooks package, or add // @bffx:hook above it", name, name)
	}
}

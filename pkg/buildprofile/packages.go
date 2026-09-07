package buildprofile

import "sort"

// ToolingPackages are CLI/dev packages omitted from minimal-mode .bffx/core vendoring.
// Runtime orchestrator builds do not import these paths.
var ToolingPackages = []string{
	"pkg/compiler",
	"pkg/generator",
	"pkg/doctor",
	"pkg/deploy",
	"pkg/mcp",
	"pkg/dev",
	"pkg/testing",
}

// runtimePackages are always vendored for minimal-mode runtime builds.
// Router static imports (addons/game/ai) remain included until build-tag splits land.
var runtimePackages = []string{
	"pkg/admin",
	"pkg/ai",
	"pkg/api",
	"pkg/app",
	"pkg/audit",
	"pkg/auth",
	"pkg/addons/ads",
	"pkg/addons/billing",
	"pkg/addons/catalog",
	"pkg/batteries",
	"pkg/cache",
	"pkg/comm",
	"pkg/errors",
	"pkg/events",
	"pkg/featureflags",
	"pkg/game",
	"pkg/i18n",
	"pkg/logger",
	"pkg/manifest",
	"pkg/observability",
	"pkg/runtimecontracts",
	"pkg/storage",
	"pkg/version",
	"pkg/worker",
}

// ResolvePackages returns sorted pkg/ paths to copy for the given mode and capabilities.
func ResolvePackages(mode string, caps Capabilities) []string {
	if mode != ModeMinimal {
		return nil // nil => copy entire pkg/ tree (full mode)
	}

	set := make(map[string]struct{}, len(runtimePackages)+len(ToolingPackages))
	for _, p := range runtimePackages {
		set[p] = struct{}{}
	}
	if caps.Wire {
		set["pkg/batteries/wire"] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ExcludedToolingForMode returns tooling paths skipped in minimal vendoring.
func ExcludedToolingForMode(mode string) []string {
	if mode != ModeMinimal {
		return nil
	}
	out := append([]string(nil), ToolingPackages...)
	sort.Strings(out)
	return out
}

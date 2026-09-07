package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/deploy"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/version"
)

func readStampedFrameworkVersion(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".bffx", "framework_version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func goModUsesVendoredBffx(root string) (bool, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false, err
	}
	return strings.Contains(string(b), "replace bffx => ./.bffx/core"), nil
}

func hasPkgUnder(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "pkg"))
	return err == nil && st.IsDir()
}

// frameworkVendoringChecks validates embedded Bffx core metadata and update readiness.
func frameworkVendoringChecks(root string, reg *manifest.Registry) []Result {
	var out []Result

	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return out
	}

	vendored, err := goModUsesVendoredBffx(root)
	if err != nil {
		out = append(out, Result{
			Name:    "Framework: go.mod",
			Status:  "warn",
			Message: fmt.Sprintf("Could not read go.mod: %v", err),
		})
		return out
	}

	if !vendored {
		out = append(out, Result{
			Name:    "Framework: vendoring",
			Status:  "ok",
			Message: "No replace bffx => ./.bffx/core in go.mod — skipping embedded-core checks.",
		})
		return out
	}

	corePkg := filepath.Join(root, ".bffx", "core", "pkg")
	if st, err := os.Stat(corePkg); err != nil || !st.IsDir() {
		out = append(out, Result{
			Name:    "Framework: vendored core",
			Status:  "fail",
			Message: fmt.Sprintf("go.mod replaces bffx with ./.bffx/core but %s is missing or not a directory. Run: bffx update framework --vendor-only (or bffx deploy init from a Bffx checkout).", corePkg),
		})
		return out
	}

	coreGoMod := filepath.Join(root, ".bffx", "core", "go.mod")
	if _, err := os.Stat(coreGoMod); err != nil {
		out = append(out, Result{
			Name:    "Framework: vendored module",
			Status:  "warn",
			Message: ".bffx/core/go.mod missing — partial vendor; re-run bffx update framework --vendor-only.",
		})
	}

	stamped := readStampedFrameworkVersion(root)
	cli := strings.TrimSpace(version.FrameworkVersion)

	if stamped == "" {
		out = append(out, Result{
			Name:    "Framework: version stamp",
			Status:  "warn",
			Message: ".bffx/framework_version is missing (legacy project or incomplete vendor). Run: bffx update framework --vendor-only",
		})
	} else if stamped != cli {
		out = append(out, Result{
			Name:    "Framework: version stamp",
			Status:  "warn",
			Message: fmt.Sprintf("Stamps differ: project=%q this CLI/framework=%q. Run: bffx update framework --vendor-only --dry-run (then without --dry-run).", stamped, cli),
		})
	} else {
		out = append(out, Result{
			Name:    "Framework: version stamp",
			Status:  "ok",
			Message: fmt.Sprintf("Embedded framework stamp matches this CLI (%s).", cli),
		})
	}

	rootHint := filepath.Join(root, ".bffx", "framework_root")
	if b, err := os.ReadFile(rootHint); err == nil {
		line := strings.TrimSpace(strings.Split(string(b), "\n")[0])
		if line != "" && !hasPkgUnder(line) {
			out = append(out, Result{
				Name:    "Framework: framework_root",
				Status:  "warn",
				Message: fmt.Sprintf(".bffx/framework_root points to %q but that path has no pkg/ directory. Fix the file or set BFFX_ROOT.", line),
			})
		} else if line != "" {
			out = append(out, Result{
				Name:    "Framework: framework_root",
				Status:  "ok",
				Message: fmt.Sprintf("framework_root points at a valid checkout: %s", line),
			})
		}
	}

	if ws := deploy.DetectWorkspaceRoot(root); ws == "" {
		out = append(out, Result{
			Name:    "Framework: update source",
			Status:  "warn",
			Message: "Could not locate a Bffx framework checkout (walk-up from project and CWD found no pkg/). Set BFFX_ROOT or add .bffx/framework_root so bffx update framework can refresh .bffx/core.",
		})
	} else {
		out = append(out, Result{
			Name:    "Framework: update source",
			Status:  "ok",
			Message: fmt.Sprintf("Framework checkout discoverable at %s", ws),
		})
	}

	if reg != nil && reg.Project != nil {
		var spec manifest.ProjectSpec
		if err := reg.Project.UnmarshalSpec(&spec); err == nil {
			auth := strings.ToLower(strings.TrimSpace(spec.App.AuthStrategy))
			if auth == "optional" || auth == "mandatory" {
				authGo := filepath.Join(root, ".bffx", "core", "pkg", "api", "handlers", "auth.go")
				b, err := os.ReadFile(authGo)
				switch {
				case err != nil:
					out = append(out, Result{
						Name:    "Framework: auth vs manifest",
						Status:  "warn",
						Message: fmt.Sprintf("authStrategy is %q but vendored auth handler is unreadable (%v). Re-vendor with: bffx update framework --vendor-only", auth, err),
					})
				case !strings.Contains(string(b), "AnonymousSession"):
					out = append(out, Result{
						Name:    "Framework: auth vs manifest",
						Status:  "warn",
						Message: fmt.Sprintf("authStrategy is %q but vendored auth handler looks outdated (no AnonymousSession). Run: bffx update framework --vendor-only", auth),
					})
				default:
					out = append(out, Result{
						Name:    "Framework: auth vs manifest",
						Status:  "ok",
						Message: fmt.Sprintf("Vendored auth stack matches authStrategy=%q (anonymous session support present).", auth),
					})
				}
			}
		}
	}

	return out
}

package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/logger"
)

// vendorSkipDirNames are never copied into vendored .bffx/core (any depth).
var vendorSkipDirNames = map[string]bool{
	"node_modules": true,
	".git":         true,
}

// vendorSkipAdminUIPaths are omitted under pkg/admin/ui-v2 when vendoring into
// generated projects. Runtime only needs dist/ (go:embed); src is build-time only
// in the framework repo or CI/Docker.
var vendorSkipAdminUIPaths = map[string]bool{
	"node_modules": true,
	"src":          true,
}


// EnsureAdminUIDist builds the admin SPA in the framework tree when admin is
// enabled and dist/ is missing or stale relative to src.
func EnsureAdminUIDist(workspaceRoot string, profile buildprofile.Profile) error {
	if !profile.Capabilities.Admin {
		return nil
	}
	uiDir := filepath.Join(workspaceRoot, "pkg", "admin", "ui-v2")
	distIndex := filepath.Join(uiDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		return nil
	}
	pkgJSON := filepath.Join(uiDir, "package.json")
	if _, err := os.Stat(pkgJSON); err != nil {
		return fmt.Errorf("admin UI enabled but %s is missing in framework checkout", pkgJSON)
	}
	logger.Info("Building admin UI (dist/ missing); running npm ci && npm run build in %s", uiDir)
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("admin UI dist missing and npm not in PATH: run scripts/build-admin-ui.sh in the Bffx repo or commit pkg/admin/ui-v2/dist")
	}
	ci := exec.Command("npm", "ci")
	ci.Dir = uiDir
	if out, err := ci.CombinedOutput(); err != nil {
		return fmt.Errorf("npm ci in admin ui-v2: %w\n%s", err, string(out))
	}
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = uiDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm run build in admin ui-v2: %w\n%s", err, string(out))
	}
	if _, err := os.Stat(distIndex); err != nil {
		return fmt.Errorf("admin UI build did not produce %s", distIndex)
	}
	logger.Info("Admin UI built successfully at %s", distIndex)
	return nil
}

// VerifyVendoredAdminDist ensures embedded admin assets exist after vendoring.
func VerifyVendoredAdminDist(vendorDir string, profile buildprofile.Profile) error {
	if !profile.Capabilities.Admin {
		return nil
	}
	distIndex := filepath.Join(vendorDir, "pkg", "admin", "ui-v2", "dist", "index.html")
	if _, err := os.Stat(distIndex); err != nil {
		return fmt.Errorf("vendored admin UI missing %s (enable admin requires ui-v2/dist; run scripts/build-admin-ui.sh in Bffx)", distIndex)
	}
	return nil
}

func copyDirFiltered(src, dst string) error {
	return copyDirFilteredRel(src, dst, "")
}

func copyDirFilteredRel(src, dst, rel string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if shouldSkipVendorEntry(rel, name) {
			continue
		}
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		entryRel := name
		if rel != "" {
			entryRel = filepath.Join(rel, name)
		}
		if entry.IsDir() {
			if err := copyDirFilteredRel(srcPath, dstPath, entryRel); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func shouldSkipVendorEntry(relFromPkg, name string) bool {
	if vendorSkipDirNames[name] {
		return true
	}
	if filepath.ToSlash(relFromPkg) == "admin/ui-v2" && vendorSkipAdminUIPaths[name] {
		return true
	}
	return false
}

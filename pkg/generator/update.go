package generator

import (
	"github.com/hangry-coder/bffx/pkg/deploy"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/version"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FrameworkUpdateOptions configures framework update commands.
type FrameworkUpdateOptions struct {
	DryRun bool
	Force  bool
}

func readFrameworkVersion(projectDir string) string {
	b, err := os.ReadFile(filepath.Join(projectDir, ".bffx", "framework_version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// UpdateFramework refreshes generated shell files, deployment templates, and vendored core.
func UpdateFramework(projectDir string, opts *FrameworkUpdateOptions) error {
	o := FrameworkUpdateOptions{}
	if opts != nil {
		o = *opts
	}

	if !o.DryRun && !o.Force && isGitDirty(projectDir) {
		return fmt.Errorf("git workspace is dirty; commit or stash changes before updating the framework (or use --dry-run to preview)")
	}

	reg, err := manifest.LoadAll(projectDir)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	projectName := reg.Project.Metadata.Name
	projVer := readFrameworkVersion(projectDir)
	cliVer := version.FrameworkVersion

	logger.Info("Project: %s", projectName)
	logger.Info("Framework version: stamped=%q CLI/tree=%q", projVer, cliVer)

	// Semver stamp can match while .bffx/core is still stale (constant not bumped every commit).
	// When stamp == CLI and not --force: skip Docker/main/tests but still re-vendor pkg/ if we can find a framework checkout.
	if !o.Force && !o.DryRun && projVer != "" && projVer == cliVer {
		workspaceRoot := deploy.DetectWorkspaceRoot(projectDir)
		if workspaceRoot == "" {
			logger.Info("Stamped version matches this CLI — skipping generated files. Cannot locate a Bffx checkout to refresh .bffx/core (set BFFX_ROOT or fix .bffx/framework_root). Use --force to regenerate main.go, Docker, and tests anyway.")
			return nil
		}
		logger.Info("Stamped version matches this CLI — skipping main.go, Dockerfiles, compose, and test scaffolding. Refreshing vendored core only from %s (use --force to re-apply those files too).", workspaceRoot)
		if err := deploy.Vendor(projectDir, workspaceRoot, nil); err != nil {
			return err
		}
		logger.Info("✅ Vendored core updated. Run `bffx doctor` to verify.")
		return nil
	}

	if o.DryRun {
		logger.Info("Dry run: no files will be modified.")
	}

	mainGoPath := "cmd/orchestrator/main.go"
	if _, err := os.Stat(filepath.Join(projectDir, "cmd", "api")); err == nil {
		mainGoPath = "cmd/api/main.go"
	}

	backups := []string{
		mainGoPath,
		"Dockerfile",
		"docker-compose.yml",
	}

	if !o.DryRun {
		for _, f := range backups {
			path := filepath.Join(projectDir, f)
			if _, err := os.Stat(path); err == nil {
				logger.Info("Backing up %s...", f)
				if err := backupFile(path); err != nil {
					return fmt.Errorf("failed to backup %s: %w", f, err)
				}
			}
		}
	} else {
		for _, f := range backups {
			path := filepath.Join(projectDir, f)
			if _, err := os.Stat(path); err == nil {
				logger.Info("[dry-run] would backup %s -> %s.bak", path, path)
			}
		}
	}

	logger.Info("Patching orchestrator main.go...")
	if err := GenerateOrchestratorMain(projectDir, o.DryRun); err != nil {
		return err
	}

	workspaceRoot := deploy.DetectWorkspaceRoot(projectDir)
	if workspaceRoot == "" {
		logger.Warn("Could not locate Bffx framework root (no pkg/ when walking up from project or CWD; .bffx/framework_root missing/invalid; BFFX_ROOT unset). Vendored .bffx/core will not be refreshed.")
	}

	initOpts := &deploy.InitOptions{DryRun: o.DryRun}
	logger.Info("Patching deployment files (Dockerfile, docker-compose)...")
	if err := deploy.Init(projectDir, workspaceRoot, initOpts); err != nil {
		return err
	}

	logger.Info("Refreshing test scaffolding...")
	if o.DryRun {
		logger.Info("[dry-run] would regenerate tests for %q", projectName)
	} else {
		if err := GenerateTests(projectDir, projectName); err != nil {
			return err
		}
	}

	if o.DryRun {
		logger.Info("✅ Dry run complete — no changes written.")
		return nil
	}

	logger.Info("✅ Framework update complete.")
	logger.Info("Please review changes and delete *.bak files once verified.")
	return nil
}

// UpdateVendorCore copies the current Bffx framework pkg/ into projectDir/.bffx/core.
func UpdateVendorCore(projectDir string, opts *FrameworkUpdateOptions) error {
	o := FrameworkUpdateOptions{}
	if opts != nil {
		o = *opts
	}

	if !o.DryRun && !o.Force && isGitDirty(projectDir) {
		return fmt.Errorf("git workspace is dirty; commit or stash changes before updating the framework (or use --dry-run to preview)")
	}

	reg, err := manifest.LoadAll(projectDir)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	projectName := reg.Project.Metadata.Name
	projVer := readFrameworkVersion(projectDir)
	cliVer := version.FrameworkVersion

	logger.Info("Vendoring BFFX framework core only for project: %s", projectName)
	logger.Info("Framework version: stamped=%q CLI/tree=%q", projVer, cliVer)

	if o.DryRun {
		logger.Info("Dry run: no files will be modified.")
	}

	workspaceRoot := deploy.DetectWorkspaceRoot(projectDir)
	if workspaceRoot == "" {
		return fmt.Errorf("could not locate Bffx framework root: set BFFX_ROOT, or run from a tree that contains pkg/, or add a valid .bffx/framework_root file pointing at your Bffx checkout")
	}

	vo := &deploy.VendorOptions{DryRun: o.DryRun}
	if err := deploy.Vendor(projectDir, workspaceRoot, vo); err != nil {
		return err
	}

	if o.DryRun {
		logger.Info("✅ Dry run complete — .bffx/core unchanged.")
		return nil
	}

	logger.Info("✅ Vendored framework refreshed at .bffx/core (from %s)", workspaceRoot)
	logger.Info("Run: cd %s && go test ./...", projectDir)
	return nil
}

func isGitDirty(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func backupFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0o644)
}

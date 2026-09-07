package buildprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

const (
	MigrationRecordFile = "packaging-migration.json"
	backupSubdir        = "backups/packaging-migration"
)

// MigrationRecord tracks an applied packaging migration for rollback.
type MigrationRecord struct {
	APIVersion string `json:"apiVersion"`
	FromMode   string `json:"fromMode"`
	ToMode     string `json:"toMode"`
	MigratedAt string `json:"migratedAt"`
	BackupDir  string `json:"backupDir"`
}

// MigrationPlan is the dry-run output for full ↔ minimal packaging changes.
type MigrationPlan struct {
	FromMode     string      `json:"fromMode"`
	ToMode       string      `json:"toMode"`
	Changes      []string    `json:"changes"`
	PackageDiff  PackageDiff `json:"packageDiff"`
	ExcludedDiff PackageDiff `json:"excludedToolingDiff"`
}

// PackageDiff describes pkg path set changes.
type PackageDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// PreflightResult captures blockers and warnings before migration.
type PreflightResult struct {
	OK          bool     `json:"ok"`
	CurrentMode string   `json:"currentMode"`
	TargetMode  string   `json:"targetMode"`
	Blockers    []string `json:"blockers,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// ModeFromSpec returns the normalized packaging mode from a project spec.
func ModeFromSpec(spec *manifest.ProjectSpec) string {
	if spec == nil {
		return ModeFull
	}
	return normalizeMode(spec.Packaging.Mode)
}

// Preflight checks whether a packaging migration can proceed.
func Preflight(reg *manifest.Registry, targetMode string) PreflightResult {
	targetMode = normalizeMode(targetMode)
	current := ModeFromSpec(reg.ProjectSpec())

	out := PreflightResult{
		CurrentMode: current,
		TargetMode:  targetMode,
		OK:          true,
	}

	if targetMode != ModeFull && targetMode != ModeMinimal {
		out.OK = false
		out.Blockers = append(out.Blockers, fmt.Sprintf("unsupported target mode %q (use full or minimal)", targetMode))
		return out
	}
	if current == targetMode {
		out.OK = false
		out.Blockers = append(out.Blockers, fmt.Sprintf("project already in %q mode", current))
		return out
	}

	if targetMode == ModeMinimal {
		out.Warnings = append(out.Warnings,
			"minimal mode excludes CLI tooling packages from .bffx/core vendoring; run bffx update framework --vendor-only after apply",
		)
	}
	if targetMode == ModeFull {
		out.Warnings = append(out.Warnings,
			"full mode restores whole pkg/ vendoring; .bffx/core size may increase",
		)
	}

	return out
}

// PlanMigration builds a dry-run plan for switching packaging mode.
func PlanMigration(reg *manifest.Registry, targetMode string) (*MigrationPlan, error) {
	targetMode = normalizeMode(targetMode)
	pf := Preflight(reg, targetMode)
	if !pf.OK {
		return nil, fmt.Errorf("%s", strings.Join(pf.Blockers, "; "))
	}

	fromCaps := deriveCapabilities(reg, reg.ProjectSpec())
	toSpec := *reg.ProjectSpec()
	toSpec.Packaging.Mode = targetMode
	toCaps := deriveCapabilities(reg, &toSpec)

	fromPackages := packageSet(ResolvePackages(pf.CurrentMode, fromCaps))
	toPackages := packageSet(ResolvePackages(targetMode, toCaps))

	plan := &MigrationPlan{
		FromMode: pf.CurrentMode,
		ToMode:   targetMode,
		Changes: []string{
			fmt.Sprintf("spec.packaging.mode: %q -> %q", pf.CurrentMode, targetMode),
			"refresh .bffx/build-profile.json from manifests",
		},
		PackageDiff: diffSets(fromPackages, toPackages),
		ExcludedDiff: PackageDiff{
			Added:   diffAdded(packageSet(ExcludedToolingForMode(pf.CurrentMode)), packageSet(ExcludedToolingForMode(targetMode))),
			Removed: diffAdded(packageSet(ExcludedToolingForMode(targetMode)), packageSet(ExcludedToolingForMode(pf.CurrentMode))),
		},
	}

	if targetMode == ModeMinimal {
		plan.Changes = append(plan.Changes,
			fmt.Sprintf("vendor %d runtime pkg paths (exclude %d tooling paths)", len(toPackages), len(ToolingPackages)),
		)
	} else {
		plan.Changes = append(plan.Changes, "vendor entire pkg/ tree on next bffx update framework --vendor-only")
	}

	return plan, nil
}

// ApplyMigration switches packaging mode, backs up files, and refreshes the build profile.
func ApplyMigration(root string, reg *manifest.Registry, targetMode string, dryRun bool) (*MigrationPlan, error) {
	plan, err := PlanMigration(reg, targetMode)
	if err != nil {
		return nil, err
	}

	if dryRun {
		return plan, nil
	}

	backupDir, err := backupMigrationFiles(root)
	if err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}

	if err := setProjectPackagingMode(root, plan.ToMode); err != nil {
		return nil, fmt.Errorf("update project.yaml: %w", err)
	}

	manifest.InvalidateLoadAllCache(root)
	reg, err = manifest.LoadAll(root)
	if err != nil {
		return nil, fmt.Errorf("reload manifests: %w", err)
	}

	graphHash := readGraphHashFile(root)
	prof, err := Derive(reg, DeriveOptions{GraphHash: graphHash})
	if err != nil {
		return nil, fmt.Errorf("derive profile: %w", err)
	}
	if err := Write(root, prof); err != nil {
		return nil, fmt.Errorf("write profile: %w", err)
	}

	rec := MigrationRecord{
		APIVersion: APIVersion,
		FromMode:   plan.FromMode,
		ToMode:     plan.ToMode,
		MigratedAt: time.Now().UTC().Format(time.RFC3339),
		BackupDir:  backupDir,
	}
	if err := WriteMigrationRecord(root, &rec); err != nil {
		return nil, fmt.Errorf("write migration record: %w", err)
	}

	return plan, nil
}

// RollbackMigration restores project.yaml and build profile from the last migration backup.
func RollbackMigration(root string) error {
	rec, err := LoadMigrationRecord(root)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("no packaging migration record at .bffx/%s", MigrationRecordFile)
	}

	projectBackup := filepath.Join(rec.BackupDir, "project.yaml")
	profileBackup := filepath.Join(rec.BackupDir, Filename)

	if b, err := os.ReadFile(projectBackup); err != nil {
		return fmt.Errorf("read backup project.yaml: %w", err)
	} else if err := os.WriteFile(filepath.Join(root, "bffx", "project.yaml"), b, 0o644); err != nil {
		return fmt.Errorf("restore project.yaml: %w", err)
	}

	if b, err := os.ReadFile(profileBackup); err == nil {
		if err := os.WriteFile(Path(root), b, 0o644); err != nil {
			return fmt.Errorf("restore build profile: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read backup profile: %w", err)
	} else if err := os.Remove(Path(root)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove profile after rollback: %w", err)
	}

	if err := os.Remove(migrationRecordPath(root)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear migration record: %w", err)
	}

	manifest.InvalidateLoadAllCache(root)
	return nil
}

// LoadMigrationRecord reads .bffx/packaging-migration.json when present.
func LoadMigrationRecord(root string) (*MigrationRecord, error) {
	if root == "" {
		root = "."
	}
	b, err := os.ReadFile(migrationRecordPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rec MigrationRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("parse migration record: %w", err)
	}
	return &rec, nil
}

// WriteMigrationRecord writes the rollback pointer file.
func WriteMigrationRecord(root string, rec *MigrationRecord) error {
	if rec == nil {
		return fmt.Errorf("nil migration record")
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Join(root, ".bffx"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(migrationRecordPath(root), b, 0o644)
}

func migrationRecordPath(root string) string {
	return filepath.Join(root, ".bffx", MigrationRecordFile)
}

func backupMigrationFiles(root string) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	backupDir := filepath.Join(root, ".bffx", backupSubdir, stamp)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}

	projectPath := filepath.Join(root, "bffx", "project.yaml")
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(backupDir, "project.yaml"), data, 0o644); err != nil {
		return "", err
	}

	if b, err := os.ReadFile(Path(root)); err == nil {
		if err := os.WriteFile(filepath.Join(backupDir, Filename), b, 0o644); err != nil {
			return "", err
		}
	}

	return backupDir, nil
}

func setProjectPackagingMode(root, mode string) error {
	path := filepath.Join(root, "bffx", "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}

	spec, ok := asStringMap(doc["spec"])
	if !ok {
		spec = map[string]any{}
		doc["spec"] = spec
	}
	packaging, ok := asStringMap(spec["packaging"])
	if !ok {
		packaging = map[string]any{}
		spec["packaging"] = packaging
	}
	packaging["mode"] = normalizeMode(mode)

	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func asStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func readGraphHashFile(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".bffx", "graph.hash"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func packageSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		set[p] = struct{}{}
	}
	return set
}

func diffSets(from, to map[string]struct{}) PackageDiff {
	return PackageDiff{
		Added:   diffAdded(from, to),
		Removed: diffAdded(to, from),
	}
}

func diffAdded(from, to map[string]struct{}) []string {
	var out []string
	for p := range to {
		if _, ok := from[p]; !ok {
			out = append(out, p)
		}
	}
	sortStrings(out)
	return out
}

func sortStrings(ss []string) {
	sort.Strings(ss)
}

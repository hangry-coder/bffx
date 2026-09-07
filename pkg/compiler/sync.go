package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

// GoBuildEnv returns os.Environ() with any GOWORK entry removed and GOWORK=off appended,
// so `go build` / `go mod tidy` run inside a nested app module are not hijacked by a
// parent repository's go.work (which would use the wrong main module, e.g. bffx).
func GoBuildEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, "GOWORK=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "GOWORK=off")
	return out
}

type graph struct {
	Project   string   `json:"project"`
	Manifests []string `json:"manifests"`
	Resources []string `json:"resources"`
	Builders  []string `json:"builders"`
	Skills    []string `json:"skills"`
	Functions []string `json:"functions"`
}

func Sync(root string, dryRun bool) ([]storage.Change, error) {
	reg, err := manifest.LoadAll(root)
	if err != nil {
		return nil, fmt.Errorf("load manifests: %w", err)
	}

	if reg.Project == nil {
		return nil, fmt.Errorf("project.yaml not found in bffx/")
	}

	var manifestPaths []string
	var resourceNames []string
	var builderNames []string
	var skillNames []string
	var functionNames []string

	for _, r := range reg.Resources {
		resourceNames = append(resourceNames, r.Metadata.Name)
		manifestPaths = append(manifestPaths, r.Path)
	}
	for _, b := range reg.Builders {
		builderNames = append(builderNames, b.Metadata.Name)
		manifestPaths = append(manifestPaths, b.Path)
	}
	for _, p := range reg.Policies {
		manifestPaths = append(manifestPaths, p.Path)
	}
	for _, s := range reg.Skills {
		skillNames = append(skillNames, s.Metadata.Name)
		manifestPaths = append(manifestPaths, s.Path)
	}
	for _, f := range reg.Functions {
		functionNames = append(functionNames, f.Metadata.Name)
		manifestPaths = append(manifestPaths, f.Path)
	}
	var actionNames []string
	for _, a := range reg.Actions {
		actionNames = append(actionNames, a.Metadata.Name)
		manifestPaths = append(manifestPaths, a.Path)
	}
	for _, p := range reg.Pipelines {
		manifestPaths = append(manifestPaths, p.Path)
	}
	manifestPaths = append(manifestPaths, reg.Project.Path)

	// Normalize paths for graph
	for i, p := range manifestPaths {
		rel, _ := filepath.Rel(root, p)
		manifestPaths[i] = rel
	}
	sort.Strings(manifestPaths)

	g := graph{
		Project:   reg.Project.Metadata.Name,
		Manifests: manifestPaths,
		Resources: resourceNames,
		Builders:  builderNames,
		Skills:    skillNames,
		Functions: functionNames,
	}

	graphBytes, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal graph: %w", err)
	}

	digest := sha256.Sum256(graphBytes)
	hash := hex.EncodeToString(digest[:])

	diagnostics := map[string]any{
		"status":    "ok",
		"manifests": len(manifestPaths),
		"resources": resourceNames,
		"builders":  builderNames,
		"skills":    skillNames,
		"functions": functionNames,
		"warnings":  reg.Warnings,
	}
	diagBytes, _ := json.MarshalIndent(diagnostics, "", "  ")

	openAPI, err := buildOpenAPI(reg)
	if err != nil {
		return nil, fmt.Errorf("build openapi: %w", err)
	}

	plan := map[string]any{
		"steps": []string{
			"discover manifests",
			"validate schemas",
			"build graph",
			"emit build profile",
			"emit diagnostics",
			"emit openapi",
			"emit generated artifacts",
		},
		"dryRun":     dryRun,
		"idempotent": true,
		"summary":    fmt.Sprintf("%d manifests synced", len(manifestPaths)),
	}
	planBytes, _ := json.MarshalIndent(plan, "", "  ")

	if dryRun {
		fmt.Println("--- DRY RUN PLAN ---")
		fmt.Printf("Project: %s\n", reg.Project.Metadata.Name)
		fmt.Printf("Graph Hash: %s\n", hash)
		fmt.Printf("Manifests: %d\n", len(manifestPaths))
		fmt.Printf("Resources: %s\n", strings.Join(resourceNames, ", "))
		fmt.Printf("Builders: %s\n", strings.Join(builderNames, ", "))
		if len(skillNames) > 0 {
			fmt.Printf("Skills: %s\n", strings.Join(skillNames, ", "))
		}
		if len(functionNames) > 0 {
			fmt.Printf("Functions: %s\n", strings.Join(functionNames, ", "))
		}
		if len(reg.Warnings) > 0 {
			fmt.Printf("Warnings: %v\n", strings.Join(reg.Warnings, ", "))
		}
		return nil, nil
	}

	outDir := filepath.Join(root, ".bffx")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("make output dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), graphBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write graph: %w", err)
	}

	adminG, err := buildAdminGraph(reg)
	if err != nil {
		return nil, fmt.Errorf("build admin graph: %w", err)
	}
	// Re-apply operator nav pins after BuildAdminGraph (which reconciles with empty prefs).
	if p := loadAdminNavPrefs(root); len(p.Resources) > 0 || len(p.Features) > 0 {
		manifest.ApplyNavPreferences(adminG, p)
		manifest.ReconcileAdminMenu(adminG, reg, p)
	}
	adminGraphBytes, err := json.MarshalIndent(adminG, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal admin graph: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "admin-graph.json"), adminGraphBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write admin graph: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "graph.hash"), []byte(hash+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write graph hash: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "diagnostics.json"), diagBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write diagnostics: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "openapi.json"), openAPI, 0o644); err != nil {
		return nil, fmt.Errorf("write openapi: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "compile-plan.json"), planBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write compile plan: %w", err)
	}

	profile, err := buildprofile.Derive(reg, buildprofile.DeriveOptions{GraphHash: hash})
	if err != nil {
		return nil, fmt.Errorf("derive build profile: %w", err)
	}
	if err := buildprofile.Write(root, profile); err != nil {
		return nil, fmt.Errorf("write build profile: %w", err)
	}

	// Reconcile Store (Auto-Migrations)
	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		return nil, fmt.Errorf("unmarshal project spec: %w", err)
	}
	if _, err := storage.NewStore(root, &projectSpec, reg); err != nil {
		return nil, fmt.Errorf("init store for sync: %w", err)
	}
	// Check for schema drift (non-destructive) only after versioned migrations are adopted.
	migrationsDir := filepath.Join(root, "migrations")
	if _, err := os.Stat(filepath.Join(root, "db", "migrations")); err == nil {
		migrationsDir = filepath.Join(root, "db", "migrations")
	}
	if schema.MigrationsAdopted(migrationsDir) {
		snap, err := schema.LoadSnapshot(filepath.Join(migrationsDir, "schema.json"))
		if err != nil {
			logger.Warn("Failed to load schema snapshot: %v", err)
		} else {
			ops := schema.Diff(schema.FromRegistry(reg), snap)
			if len(ops) > 0 {
				logger.Warn("Schema drift detected: %d changes pending. Run: bffx migrate plan", len(ops))
				for _, op := range ops {
					if op.Column != "" {
						logger.Warn("  %s %s.%s", op.Kind, op.Table, op.Column)
					} else {
						logger.Warn("  %s %s", op.Kind, op.Table)
					}
				}
			}
		}
	} else if projectSpec.Store.Mode == "sqlite" || projectSpec.Store.Mode == "postgres" {
		logger.Info("Versioned migrations not adopted yet (no schema.json or *.up.sql in %s). Skipping schema drift diff. Run `bffx migrate init` after the database exists to baseline migrations.", migrationsDir)
	}

	if err := SyncWire(root, reg, &projectSpec); err != nil {
		return nil, fmt.Errorf("sync wire: %w", err)
	}

	if err := emitGoArtifacts(root, actionNames, reg); err != nil {
		return nil, fmt.Errorf("emit go artifacts: %w", err)
	}
	if err := generator.EmitTypedModels(root, reg); err != nil {
		return nil, fmt.Errorf("emit typed models: %w", err)
	}

	// Auto-run go mod tidy to ensure orchestrator has resolving imports
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = root
	cmd.Env = GoBuildEnv()
	cmd.Run()

	return nil, nil
}

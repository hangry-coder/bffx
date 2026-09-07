package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

// handleMigrateLayout migrates a legacy hierarchical project layout to v2 vertical-slice layout.
func handleMigrateLayout(root string, args []string) {
	dryRun := true
	for _, a := range args {
		if a == "--apply" {
			dryRun = false
		}
	}

	fmt.Printf("BFFX layout migration utility for: %s\n", root)
	if dryRun {
		fmt.Println("Running in DRY-RUN mode. No files will be modified. Use '--apply' to execute.")
	}
	fmt.Println()

	// 1. Verify project exists and load project.yaml
	projectYamlPath := filepath.Join(root, "bffx", "project.yaml")
	if _, err := os.Stat(projectYamlPath); os.IsNotExist(err) {
		fmt.Printf("Error: bffx/project.yaml not found at %s. Are you in a BFFX project root?\n", root)
		os.Exit(1)
	}

	// Load Project manifest
	reg, err := manifest.LoadAll(root)
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	if reg.Project == nil {
		fmt.Println("Error: No Project manifest parsed.")
		os.Exit(1)
	}

	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		fmt.Printf("Error parsing project spec: %v\n", err)
		os.Exit(1)
	}

	if projectSpec.Layout == "v2" {
		fmt.Println("Project is already in 'v2' layout. No migration needed.")
		return
	}

	// 2. Walk bffx/ directory and find all yaml files (excluding project.yaml)
	type manifestMove struct {
		from string
		to   string
	}
	var manifestMoves []manifestMove

	// Map to track resource/action names to feature/group mapping for hooks resolution
	nameToFeature := make(map[string]string)

	err = filepath.Walk(filepath.Join(root, "bffx"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if path == projectYamlPath {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read manifest kind and group
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var m struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				Group string `yaml:"group"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(data, &m); err != nil {
			// Skip invalid manifests
			return nil
		}

		feature := m.Spec.Group
		if feature == "" {
			feature = "app"
		}
		feature = strings.ToLower(feature)

		if m.Metadata.Name != "" {
			nameToFeature[strings.ToLower(m.Metadata.Name)] = feature
		}

		var subDir string
		switch m.Kind {
		case "Resource", "Policy", "Service", "Stream":
			subDir = "manifests"
		case "Builder":
			subDir = "builders"
		case "Action":
			subDir = "actions"
		case "Screen":
			subDir = "screens"
		case "Template":
			subDir = "templates"
		case "CronJob":
			subDir = "cronjobs"
		case "Blueprint":
			subDir = "blueprints"
		default:
			subDir = "manifests"
		}

		destPath := filepath.Join(root, "internal", "features", feature, subDir, filepath.Base(path))
		manifestMoves = append(manifestMoves, manifestMove{from: path, to: destPath})
		return nil
	})
	if err != nil {
		fmt.Printf("Error analyzing manifests: %v\n", err)
		os.Exit(1)
	}

	// 3. Resolve hook files
	var hookMoves []manifestMove
	hooksDir := filepath.Join(root, "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		entries, _ := os.ReadDir(hooksDir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			fromPath := filepath.Join(hooksDir, e.Name())
			baseName := strings.TrimSuffix(e.Name(), ".go")
			
			// Find feature matching the baseName
			feature, exists := nameToFeature[strings.ToLower(baseName)]
			if !exists {
				// Fallback to app
				feature = "app"
			}

			destPath := filepath.Join(root, "internal", "features", feature, "hooks", e.Name())
			hookMoves = append(hookMoves, manifestMove{from: fromPath, to: destPath})
		}
	}

	// 4. Other directory moves (migrations and i18n)
	var dirMoves []manifestMove
	legacyMigDir := filepath.Join(root, "migrations")
	if _, err := os.Stat(legacyMigDir); err == nil {
		dirMoves = append(dirMoves, manifestMove{from: legacyMigDir, to: filepath.Join(root, "db", "migrations")})
	}
	legacyI18nDir := filepath.Join(root, "i18n")
	if _, err := os.Stat(legacyI18nDir); err == nil {
		dirMoves = append(dirMoves, manifestMove{from: legacyI18nDir, to: filepath.Join(root, "assets", "i18n")})
	}

	// 5. Output the execution plan
	fmt.Println("--- MIGRATION PLAN ---")
	for _, m := range manifestMoves {
		fmt.Printf("  [move] %s -> %s\n", relPath(root, m.from), relPath(root, m.to))
	}
	for _, h := range hookMoves {
		fmt.Printf("  [move] %s -> %s\n", relPath(root, h.from), relPath(root, h.to))
	}
	for _, d := range dirMoves {
		fmt.Printf("  [move] %s/* -> %s/*\n", relPath(root, d.from), relPath(root, d.to))
	}
	fmt.Printf("  [update] %s -> set spec.layout: v2\n", relPath(root, projectYamlPath))
	fmt.Println("----------------------")
	fmt.Println()

	if dryRun {
		fmt.Println("Dry-run complete. Run 'bffx migrate layout --apply' to make changes.")
		return
	}

	// 6. Execute moves
	fmt.Println("Executing migration...")

	// Function to copy/rename safely
	moveFile := func(src, dst string) error {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		err := os.Rename(src, dst)
		if err != nil {
			// Fallback to copy if rename fails (e.g. cross-volume)
			if errCopy := copyFile(src, dst); errCopy != nil {
				return fmt.Errorf("rename failed: %v, copy failed: %v", err, errCopy)
			}
			_ = os.Remove(src)
		}
		return nil
	}

	// Move manifests
	for _, m := range manifestMoves {
		if err := moveFile(m.from, m.to); err != nil {
			fmt.Printf("Failed to move manifest %s: %v\n", m.from, err)
			os.Exit(1)
		}
	}

	// Move hooks
	for _, h := range hookMoves {
		if err := moveFile(h.from, h.to); err != nil {
			fmt.Printf("Failed to move hook file %s: %v\n", h.from, err)
			os.Exit(1)
		}
	}

	// Move directories
	for _, d := range dirMoves {
		if err := os.MkdirAll(filepath.Dir(d.to), 0o755); err != nil {
			fmt.Printf("Failed to create dir %s: %v\n", filepath.Dir(d.to), err)
			os.Exit(1)
		}
		// If destination already exists, we must merge or rename
		if err := os.Rename(d.from, d.to); err != nil {
			// Fallback copy dir
			if errCopy := copyDir(d.from, d.to); errCopy != nil {
				fmt.Printf("Failed to copy dir %s -> %s: %v\n", d.from, d.to, errCopy)
				os.Exit(1)
			}
			_ = os.RemoveAll(d.from)
		}
	}

	// Update project.yaml layout to v2
	projData, err := os.ReadFile(projectYamlPath)
	if err != nil {
		fmt.Printf("Failed to read project.yaml: %v\n", err)
		os.Exit(1)
	}

	var projMap map[string]any
	if err := yaml.Unmarshal(projData, &projMap); err != nil {
		fmt.Printf("Failed to parse project.yaml: %v\n", err)
		os.Exit(1)
	}

	specVal, ok := projMap["spec"].(map[string]any)
	if !ok {
		specVal = make(map[string]any)
		projMap["spec"] = specVal
	}
	specVal["layout"] = "v2"

	newProjData, err := yaml.Marshal(projMap)
	if err != nil {
		fmt.Printf("Failed to marshal updated project.yaml: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(projectYamlPath, newProjData, 0o644); err != nil {
		fmt.Printf("Failed to write updated project.yaml: %v\n", err)
		os.Exit(1)
	}

	// Clean up empty directories
	_ = os.Remove(hooksDir)
	_ = os.Remove(legacyI18nDir)
	_ = os.Remove(legacyMigDir)

	// Clean up empty directories inside bffx/ except project.yaml
	entries, _ := os.ReadDir(filepath.Join(root, "bffx"))
	for _, e := range entries {
		if e.IsDir() {
			_ = os.RemoveAll(filepath.Join(root, "bffx", e.Name()))
		} else if e.Name() != "project.yaml" {
			_ = os.Remove(filepath.Join(root, "bffx", e.Name()))
		}
	}

	fmt.Println()
	fmt.Println("Migration executed successfully!")
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'bffx sync' to rebuild generated schema/routes registry.")
	fmt.Println("  2. Run 'go test ./... -short' to verify vertical-slices compile and pass.")
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func ScaffoldNewProject(root, name string, opts ProjectOptions) error {
	if opts.Layout == "" {
		opts.Layout = LayoutV2
	}
	projectDir := filepath.Join(root, name)

	// 1. Create directories
	if err := scaffoldDirectories(projectDir, opts); err != nil {
		return err
	}

	// 2. Default options
	normalized, err := normalizeStoreMode(opts.StoreMode)
	if err != nil {
		return err
	}
	opts.StoreMode = normalized
	if opts.Batteries != nil && opts.Batteries.Store != "" {
		if s, err := normalizeStoreMode(opts.Batteries.Store); err == nil {
			opts.Batteries.Store = s
		}
	}

	// 3. Generate Configs
	if err := generateConfigs(projectDir, name, opts); err != nil {
		return err
	}

	// 4. Generate Dockerfiles
	if err := generateDockerfiles(projectDir, name, opts); err != nil {
		return err
	}

	// 5. Generate Core Resources
	if err := generateCoreResources(projectDir, name, opts); err != nil {
		return err
	}

	// 6. Generate UI & Builders
	if err := generateUI(projectDir, name, opts); err != nil {
		return err
	}

	// 7. Generate i18n
	if err := generateI18n(projectDir, name, opts); err != nil {
		return err
	}

	// Generate Email Templates (welcome email)
	if err := generateEmailTemplates(projectDir, name, opts); err != nil {
		return err
	}

	// 8. Generate Onboarding
	if err := generateOnboarding(projectDir, name, opts); err != nil {
		return err
	}

	// 8b. Generate Admin defaults
	if err := GenerateAdminDefaults(projectDir, opts); err != nil {
		return err
	}

	// 9. Generate Filesystem basics (README, go.mod, etc.)
	if err := generateBasics(projectDir, name, opts); err != nil {
		return err
	}

	return nil
}

func EnableAdmin(root string) error {
	reg, err := manifest.LoadAll(root)
	if err != nil {
		return fmt.Errorf("load manifests: %w", err)
	}

	projectPath := reg.Project.Path
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return fmt.Errorf("read project manifest: %w", err)
	}

	content := string(data)
	updated := false
	if strings.Contains(content, "  admin:") {
		if strings.Contains(content, "    enabled: false") {
			content = strings.ReplaceAll(content, "    enabled: false", "    enabled: true")
			updated = true
		}
	} else {
		lines := strings.Split(content, "\n")
		inSpec := false
		lastSpecLine := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "spec:") {
				inSpec = true
			}
			if inSpec && (strings.HasPrefix(line, "  ") || line == "") {
				if strings.TrimSpace(line) != "" {
					lastSpecLine = i
				}
			} else if inSpec && line != "" && !strings.HasPrefix(line, " ") {
				break
			}
		}

		if lastSpecLine >= 0 {
			adminBlock := []string{"  admin:", "    enabled: true"}
			newLines := append(lines[:lastSpecLine+1], append(adminBlock, lines[lastSpecLine+1:]...)...)
			content = strings.Join(newLines, "\n")
		} else {
			content += "  admin:\n    enabled: true\n"
		}
		updated = true
	}

	if updated {
		if err := os.WriteFile(projectPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("update project manifest: %w", err)
		}
		manifest.InvalidateLoadAllCache(root)
	}

	if err := BackfillAdminManifests(root); err != nil {
		return fmt.Errorf("backfill admin manifests: %w", err)
	}

	return nil
}

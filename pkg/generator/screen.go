package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ScreenOptions struct {
	NavType      string
	Icon         string
	Order        int
	RequiresAuth string
	Group        string
	Layout       LayoutType
}

func GenerateScreen(root, name string, opts ScreenOptions) error {
	if name == "" {
		return errors.New("screen name is required")
	}

	paths := GetPaths(root, opts.Group, opts.Layout)
	if err := os.MkdirAll(paths.Screens, 0o755); err != nil {
		return fmt.Errorf("create screens directory: %w", err)
	}

	filePath := filepath.Join(paths.Screens, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("screen manifest already exists: %s", filePath)
	}

	if opts.NavType == "" {
		opts.NavType = "bottom"
	}
	if opts.Group == "" {
		opts.Group = "mobile"
	}

	auth := opts.RequiresAuth
	if auth == "" {
		auth = "optional"
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Screen",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  group: " + opts.Group,
		"  name: " + name,
		"  nav_type: " + opts.NavType,
		"  icon: " + opts.Icon,
		"  order: " + fmt.Sprintf("%d", opts.Order),
		"  requires_auth: " + opts.RequiresAuth,
		"  route:",
		"    method: GET",
		"    path: /api/v1/screens/" + strings.ToLower(name),
		"    auth: " + auth,
		"  sources: []",
		"  output: {}",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write screen manifest: %w", err)
	}

	fmt.Printf("scaffolded screen in %s\n", filePath)

	if err := EnsureAdminManifestForScreen(root, name, opts.Layout); err != nil {
		return fmt.Errorf("generate admin screen manifest: %w", err)
	}

	return nil
}

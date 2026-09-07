package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

type AdminManifestOptions struct {
	Resource     string
	Label        string
	Parent       string
	Priority     int
	Pin          bool
	IndexColumns []string
	ShowAttrs    []string
	FormExclude  []string
}

func skipAdminManifest() bool {
	for _, arg := range os.Args {
		if arg == "--no-admin-manifest" {
			return true
		}
	}
	return false
}

func EnsureAdminManifest(projectDir, kind, name string, layout LayoutType, opts AdminManifestOptions) error {
	if skipAdminManifest() {
		return nil
	}
	// If project does not have admin enabled, do not generate manifests.
	reg, err := manifest.LoadAll(projectDir)
	if err == nil && reg.Project != nil {
		if !reg.ProjectSpec().Admin.Enabled {
			return nil
		}
	}

	path := adminManifestPath(projectDir, kind, name, layout)
	// Never overwrite user edits
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var content string
	switch kind {
	case "AdminSite":
		content = strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: AdminSite",
			"metadata:",
			"  name: " + name,
			"spec:",
			"  title: \"" + strings.Title(name) + " Console\"",
			"  theme: system",
			"  default_per_page: 25",
			"  session:",
			"    dev_unlimited: true",
			"    # max_age: 8h",
			"    # idle_timeout: 30m",
			"  menu:",
			"    - label: Dashboard",
			"      page: dashboard",
			"      priority: 0",
			"    - label: Feature Flags",
			"      page: feature-flags",
			"      priority: 5",
			"    - label: Users",
			"      page: users",
			"      priority: 8",
			"    - label: Monitoring",
			"      priority: 50",
			"      children:",
			"        - label: Performance",
			"          page: performance",
			"          priority: 0",
			"    - label: Resources",
			"      priority: 80",
			"    - label: App Features",
			"      priority: 70",
			"    - label: Settings",
			"      priority: 90",
			"  features:",
			"    app_features: true",
			"    feature_flags: true",
			"    jobs: true",
			"    liveops: true",
			"    api_docs: true",
			"  export:",
			"    csv_enabled: true",
		}, "\n")
	case "AdminDashboard":
		content = strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: AdminDashboard",
			"metadata:",
			"  name: " + name,
			"spec:",
			"  widgets:",
			"    - type: metric",
			"      title: Active users (24h)",
			"      query:",
			"        resource: User",
			"        aggregate: count",
			"        where:",
			"          status: active",
		}, "\n")
	case "AdminScreen":
		content = strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: AdminScreen",
			"metadata:",
			"  name: " + name + "Admin",
			"spec:",
			"  screen: " + name,
			"  kill_switch_panel: true",
		}, "\n")
	case "AdminAction":
		content = strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: AdminAction",
			"metadata:",
			"  name: " + name + "Admin",
			"spec:",
			"  action: " + name,
			"  observe_card: true",
		}, "\n")
	case "AdminResource":
		pinStr := "false"
		if opts.Pin {
			pinStr = "true"
		}
		var colsStr string
		if len(opts.IndexColumns) > 0 {
			colsStr = "\n    columns:"
			for _, col := range opts.IndexColumns {
				colsStr += "\n      - " + col
			}
		}
		var attrsStr string
		if len(opts.ShowAttrs) > 0 {
			attrsStr = "\n  show:\n    attributes:"
			for _, attr := range opts.ShowAttrs {
				attrsStr += "\n      - " + attr
			}
		}
		var formExcludeStr string
		if len(opts.FormExclude) > 0 {
			formExcludeStr = "\n  form:\n    exclude:"
			for _, excl := range opts.FormExclude {
				formExcludeStr += "\n      - " + excl
			}
		}

		content = fmt.Sprintf(`apiVersion: bffx.io/v1alpha1
kind: AdminResource
metadata:
  name: %sAdmin
spec:
  resource: %s
  enabled: true
  menu:
    label: %s
    parent: %s
    priority: %d
    pin: %s%s%s%s
`, name, opts.Resource, opts.Label, opts.Parent, opts.Priority, pinStr, colsStr, attrsStr, formExcludeStr)
	default:
		return fmt.Errorf("unknown admin manifest kind: %s", kind)
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

func adminManifestPath(root string, kind string, name string, layout LayoutType) string {
	if layout == LayoutV2 {
		dir := filepath.Join(root, "internal", "features", "admin", "manifests")
		switch kind {
		case "AdminScreen":
			return filepath.Join(dir, "screens", strings.ToLower(name)+".yaml")
		case "AdminAction":
			return filepath.Join(dir, "actions", strings.ToLower(name)+".yaml")
		case "AdminPage":
			return filepath.Join(dir, "pages", strings.ToLower(name)+".yaml")
		case "AdminResource":
			return filepath.Join(dir, "resources", strings.ToLower(name)+".yaml")
		default:
			return filepath.Join(dir, strings.ToLower(name)+".yaml")
		}
	}

	dir := filepath.Join(root, "bffx", "admin")
	switch kind {
	case "AdminScreen":
		return filepath.Join(dir, "screens", strings.ToLower(name)+".yaml")
	case "AdminAction":
		return filepath.Join(dir, "actions", strings.ToLower(name)+".yaml")
	case "AdminPage":
		return filepath.Join(dir, "pages", strings.ToLower(name)+".yaml")
	default:
		return filepath.Join(dir, strings.ToLower(name)+".yaml")
	}
}

func GenerateAdminDefaults(projectDir string, opts ProjectOptions) error {
	if !opts.AdminEnabled {
		return nil
	}

	// 1. Scaffold site.yaml
	if err := EnsureAdminManifest(projectDir, "AdminSite", "default", opts.Layout, AdminManifestOptions{}); err != nil {
		return err
	}

	// 2. Scaffold dashboard.yaml
	if err := EnsureAdminManifest(projectDir, "AdminDashboard", "main", opts.Layout, AdminManifestOptions{}); err != nil {
		return err
	}

	// 3. Scaffold core resources admin stubs
	// User Admin Resource
	userOpts := AdminManifestOptions{
		Resource:     "User",
		Label:        "Users",
		Parent:       "system",
		Priority:     10,
		Pin:          true,
		IndexColumns: []string{"email", "name", "role", "status", "created_at"},
		ShowAttrs:    []string{"email", "name", "role", "status", "device_id", "created_at"},
		FormExclude:  []string{"password", "otp_code"},
	}
	if err := EnsureAdminManifest(projectDir, "AdminResource", "User", opts.Layout, userOpts); err != nil {
		return err
	}

	// AdminUser Admin Resource
	adminUserOpts := AdminManifestOptions{
		Resource:     "AdminUser",
		Label:        "Admin Users",
		Parent:       "admin",
		Priority:     20,
		Pin:          true,
		IndexColumns: []string{"email", "role"},
		ShowAttrs:    []string{"email", "role"},
		FormExclude:  []string{"password"},
	}
	if err := EnsureAdminManifest(projectDir, "AdminResource", "AdminUser", opts.Layout, adminUserOpts); err != nil {
		return err
	}

	return nil
}

func isSensitiveField(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "password") ||
		strings.Contains(n, "otp") ||
		strings.Contains(n, "secret") ||
		strings.Contains(n, "token") ||
		strings.Contains(n, "hash") ||
		n == "key"
}

func EnsureAdminManifestForResource(projectDir, name string, fields []Field, layout LayoutType) error {
	reg, err := manifest.LoadAll(projectDir)
	if err == nil && reg.Project != nil {
		spec := reg.ProjectSpec()
		if !spec.Admin.Enabled {
			return nil
		}
		if spec.Admin.AutoManifests != nil && !*spec.Admin.AutoManifests {
			return nil
		}
	}

	var cols []string
	var attrs []string
	var excl []string

	for _, f := range fields {
		if isSensitiveField(f.Name) {
			excl = append(excl, f.Name)
		} else {
			attrs = append(attrs, f.Name)
			if len(cols) < 6 {
				cols = append(cols, f.Name)
			}
		}
	}

	label := name
	if strings.HasSuffix(strings.ToLower(name), "y") && !strings.HasSuffix(strings.ToLower(name), "ay") && !strings.HasSuffix(strings.ToLower(name), "ey") && !strings.HasSuffix(strings.ToLower(name), "oy") && !strings.HasSuffix(strings.ToLower(name), "uy") {
		label = name[:len(name)-1] + "ies"
	} else if !strings.HasSuffix(strings.ToLower(name), "s") {
		label = name + "s"
	}

	opts := AdminManifestOptions{
		Resource:     name,
		Label:        label,
		Parent:       "app",
		Priority:     100,
		Pin:          false,
		IndexColumns: cols,
		ShowAttrs:    attrs,
		FormExclude:  excl,
	}

	return EnsureAdminManifest(projectDir, "AdminResource", name, layout, opts)
}

func EnsureAdminManifestForScreen(projectDir, name string, layout LayoutType) error {
	return EnsureAdminManifest(projectDir, "AdminScreen", name, layout, AdminManifestOptions{})
}

func EnsureAdminManifestForAction(projectDir, name string, layout LayoutType) error {
	return EnsureAdminManifest(projectDir, "AdminAction", name, layout, AdminManifestOptions{})
}

func BackfillAdminManifests(root string) error {
	reg, err := manifest.LoadAll(root)
	if err != nil {
		return fmt.Errorf("load manifests: %w", err)
	}

	layout := LayoutLegacy
	if reg.ProjectSpec().Layout == "v2" {
		layout = LayoutV2
	}

	// 1. Ensure Site
	if err := EnsureAdminManifest(root, "AdminSite", "default", layout, AdminManifestOptions{}); err != nil {
		return err
	}

	// 2. Ensure Dashboard
	if err := EnsureAdminManifest(root, "AdminDashboard", "main", layout, AdminManifestOptions{}); err != nil {
		return err
	}

	// 3. Backfill resources
	for _, res := range reg.Resources {
		var resSpec manifest.ResourceSpec
		_ = res.UnmarshalSpec(&resSpec)

		var fields []Field
		for _, f := range resSpec.Fields {
			fields = append(fields, Field{Name: f.Name, Type: f.Type})
		}

		if err := EnsureAdminManifestForResource(root, res.Metadata.Name, fields, layout); err != nil {
			return err
		}
	}

	// 4. Backfill screens
	for _, scr := range reg.Screens {
		if err := EnsureAdminManifestForScreen(root, scr.Metadata.Name, layout); err != nil {
			return err
		}
	}

	// 5. Backfill actions
	for _, act := range reg.Actions {
		if err := EnsureAdminManifestForAction(root, act.Metadata.Name, layout); err != nil {
			return err
		}
	}

	return nil
}



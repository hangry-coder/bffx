package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ResourceOptions struct {
	WithHooks   bool
	ReadPolicy  string
	WritePolicy string
	Group       string // "system", "mobile", or "app" (default: "app")
	Layout      LayoutType
	// SkipBlueprint skips faker demo blueprints (e.g. AdminUser uses InitialAdmin instead).
	SkipBlueprint bool
}

func GenerateResource(root, name string, fields []Field, opts ResourceOptions) error {
	if name == "" {
		return errors.New("resource name is required")
	}
	if len(fields) == 0 {
		return errors.New("at least one field is required")
	}

	paths := GetPaths(root, opts.Group, opts.Layout)
	if err := os.MkdirAll(paths.Manifests, 0o755); err != nil {
		return fmt.Errorf("create manifests directory: %w", err)
	}

	filePath := filepath.Join(paths.Manifests, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("resource manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing resources.", filePath)
	}

	var fieldLines []string
	for _, f := range fields {
		t := f.Type
		uniqueStr := ""
		if strings.HasSuffix(t, ":unique") {
			t = strings.TrimSuffix(t, ":unique")
			uniqueStr = ", unique: true"
		}
		fieldLines = append(fieldLines, fmt.Sprintf("    - { name: %s, type: %s%s }", f.Name, t, uniqueStr))
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Resource",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  group: " + func() string {
			if opts.Group == "" {
				return "app"
			}
			return opts.Group
		}(),
		"  fields:",
		strings.Join(fieldLines, "\n"),
		"  routes:",
		"    crud: true",
		"  policy:",
		"    read: " + func() string {
			if opts.ReadPolicy == "" {
				return "authenticated"
			}
			return opts.ReadPolicy
		}(),
		"    write: " + func() string {
			if opts.WritePolicy == "" {
				return "authenticated"
			}
			return opts.WritePolicy
		}(),
		"  hooks: {}",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write resource manifest: %w", err)
	}

	if opts.WithHooks {
		if err := os.MkdirAll(paths.Hooks, 0o755); err != nil {
			return fmt.Errorf("create hooks directory: %w", err)
		}
		hookFile := filepath.Join(paths.Hooks, strings.ToLower(name)+".go")
		if _, err := os.Stat(hookFile); os.IsNotExist(err) {
			stubs := strings.Join([]string{
				"package hooks",
				"",
				"import (",
				"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
				")",
				"",
				"func BeforeCreate" + name + "(ctx *handlers.ActionContext, payload map[string]any) error {",
				"	return nil",
				"}",
				"",
				"func AfterCreate" + name + "(ctx *handlers.ActionContext, payload map[string]any) error {",
				"	return nil",
				"}",
				"",
				"func BeforeUpdate" + name + "(ctx *handlers.ActionContext, payload map[string]any) error {",
				"	return nil",
				"}",
				"",
				"func AfterUpdate" + name + "(ctx *handlers.ActionContext, payload map[string]any) error {",
				"	return nil",
				"}",
				"",
			}, "\n")
			if err := os.WriteFile(hookFile, []byte(stubs), 0o644); err != nil {
				return fmt.Errorf("write hook stubs: %w", err)
			}
			fmt.Printf("scaffolded hooks in %s\n", hookFile)
		}
	}

	if !opts.SkipBlueprint {
		if err := GenerateBlueprint(root, name, fields, opts); err != nil {
			return fmt.Errorf("generate %s blueprint: %w", name, err)
		}
		if err := GenerateResourceSpec(root, name, fields, opts); err != nil {
			return fmt.Errorf("generate %s resource spec: %w", name, err)
		}
	}

	if err := EnsureAdminManifestForResource(root, name, fields, opts.Layout); err != nil {
		return fmt.Errorf("generate admin resource manifest: %w", err)
	}

	return nil
}

func GenerateScaffold(root, name string, fields []Field, opts ResourceOptions) error {
	if err := GenerateResource(root, name, fields, opts); err != nil {
		return err
	}

	paths := GetPaths(root, opts.Group, opts.Layout)
	
	// Policy goes to manifests (resources in legacy)
	if err := os.MkdirAll(paths.Manifests, 0o755); err != nil {
		return fmt.Errorf("create manifests directory: %w", err)
	}
	policyPath := filepath.Join(paths.Manifests, strings.ToLower(name)+"_policy.yaml")
	policyDoc := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Policy",
		"metadata:",
		"  name: " + name + "Policy",
		"spec:",
		"  resource: " + name,
		"  read: " + func() string {
			if opts.ReadPolicy == "" {
				return "authenticated"
			}
			return opts.ReadPolicy
		}(),
		"  write: " + func() string {
			if opts.WritePolicy == "" {
				return "authenticated"
			}
			return opts.WritePolicy
		}(),
		"",
	}, "\n")
	if err := os.WriteFile(policyPath, []byte(policyDoc), 0o644); err != nil {
		return fmt.Errorf("write policy manifest: %w", err)
	}

	// Builder goes to builders directory
	if err := os.MkdirAll(paths.Builders, 0o755); err != nil {
		return fmt.Errorf("create builders directory: %w", err)
	}
	builderName := name + "List"
	builderPath := filepath.Join(paths.Builders, strings.ToLower(name)+"_list.yaml")
	builderDoc := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Builder",
		"metadata:",
		"  name: " + builderName,
		"spec:",
		"  route:",
		"    method: GET",
		"    path: /api/v1/" + strings.ToLower(name) + "s/list",
		"    auth: required",
		"  sources:",
		"    - resource:" + name,
		"  output:",
		"    items: source",
		"",
	}, "\n")
	if err := os.WriteFile(builderPath, []byte(builderDoc), 0o644); err != nil {
		return fmt.Errorf("write builder manifest: %w", err)
	}

	return nil
}

func ParseFields(raw []string) ([]Field, error) {
	fields := make([]Field, 0, len(raw))
	for _, token := range raw {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid field %q (expected name:type)", token)
		}
		fields = append(fields, Field{Name: strings.TrimSpace(parts[0]), Type: strings.TrimSpace(parts[1])})
	}
	return fields, nil
}

func GenerateBlueprint(root, name string, fields []Field, opts ResourceOptions) error {
	paths := GetPaths(root, opts.Group, opts.Layout)
	os.MkdirAll(paths.Blueprints, 0o755)

	var fieldLines []string
	for _, f := range fields {
		val := FakeValue(name, f.Name, f.Type)

		// Ensure strings are properly formatted for YAML if they have spaces/special chars
		if f.Type == "" || f.Type == "string" {
			val = fmt.Sprintf("%q", val)
		}
		fieldLines = append(fieldLines, fmt.Sprintf("    %s: %s", f.Name, val))
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Blueprint",
		"metadata:",
		"  name: " + name + "Default",
		"spec:",
		"  resource: " + name,
		"  fields:",
		strings.Join(fieldLines, "\n"),
		"",
	}, "\n")

	filePath := filepath.Join(paths.Blueprints, strings.ToLower(name)+".yaml")
	return os.WriteFile(filePath, []byte(manifest), 0o644)
}

func GenerateResourceSpec(root, name string, fields []Field, opts ResourceOptions) error {
	paths := GetPaths(root, opts.Group, opts.Layout)
	os.MkdirAll(paths.Tests, 0o755)

	spec := strings.Join([]string{
		"package specs",
		"",
		"import (",
		"	bffxtest \"github.com/hangry-coder/bffx/pkg/testing\"",
		"	\"testing\"",
		")",
		"",
		"func Test" + name + "(t *testing.T) {",
		"	bffxtest.SetT(t)",
		"	env := bffxtest.SetupEnvironment(\"..\")",
		"",
		"	bffxtest.Describe(\"" + name + " Resource\", func() {",
		"		bffxtest.It(\"should build a valid " + name + " from blueprint\", func() {",
		"			data := env.Factory.Build(\"" + name + "Default\", nil)",
		"			bffxtest.Expect(data).ToNotEqual(nil)",
		"		})",
		"",
		"		bffxtest.It(\"should create a " + name + " in the store\", func() {",
		"			record, err := env.Factory.Create(\"" + name + "Default\", nil)",
		"			bffxtest.Expect(err).ToEqual(nil)",
		"			bffxtest.Expect(record[\"id\"]).ToNotEqual(\"\")",
		"		})",
		"	})",
		"}",
		"",
	}, "\n")

	filePath := filepath.Join(paths.Tests, strings.ToLower(name)+"_spec.go")
	return os.WriteFile(filePath, []byte(spec), 0o644)
}

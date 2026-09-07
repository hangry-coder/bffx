package generator

import (
	"os"
	"path/filepath"
	"testing"
	"strings"
)

func TestGenerateResource(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-res-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	fields := []Field{{Name: "title", Type: "string"}}
	if err := GenerateResource(tmp, "Book", fields, ResourceOptions{WithHooks: true}); err != nil {
		t.Fatalf("generate resource failed: %v", err)
	}

	path := filepath.Join(tmp, "bffx/resources/book.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("resource file missing")
	}

	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "name: Book") {
		t.Errorf("resource name 'Book' not found in manifest")
	}

	// Verify hooks path
	hookPath := filepath.Join(tmp, "hooks/book.go")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Errorf("hook file missing at %s", hookPath)
	}
}

func TestGenerateScaffold(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-scaf-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	fields := []Field{{Name: "name", Type: "string"}}
	if err := GenerateScaffold(tmp, "Category", fields, ResourceOptions{}); err != nil {
		t.Fatalf("generate scaffold failed: %v", err)
	}

	// Verify Builder was created
	path := filepath.Join(tmp, "bffx/builders/category_list.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("builder file missing at %s", path)
	}
}

func TestGenerateResource_CreatesAdminResourceStub(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-res-admin-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Scaffold a minimal project config to test admin enabled
	os.MkdirAll(filepath.Join(tmp, "bffx"), 0o755)
	projectYaml := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Project",
		"metadata:",
		"  name: testapp",
		"spec:",
		"  admin:",
		"    enabled: true",
	}, "\n")
	os.WriteFile(filepath.Join(tmp, "bffx/project.yaml"), []byte(projectYaml), 0o644)

	fields := []Field{{Name: "title", Type: "string"}, {Name: "secret_token", Type: "string"}}
	if err := GenerateResource(tmp, "Book", fields, ResourceOptions{}); err != nil {
		t.Fatalf("generate resource failed: %v", err)
	}

	adminPath := filepath.Join(tmp, "bffx/admin/book.yaml")
	if _, err := os.Stat(adminPath); os.IsNotExist(err) {
		t.Fatalf("admin resource file missing at %s", adminPath)
	}

	content, _ := os.ReadFile(adminPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "resource: Book") {
		t.Errorf("expected 'resource: Book' in admin manifest")
	}
	if !strings.Contains(contentStr, "exclude:\n      - secret_token") {
		t.Errorf("expected 'secret_token' to be excluded in form, content:\n%s", contentStr)
	}
}


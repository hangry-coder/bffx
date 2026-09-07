package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_AdminManifestsV2(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, "internal", "features", "admin", "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "bffx")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.yaml"), []byte(`apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: test
spec:
  admin:
    enabled: true
  layout: v2
`), 0o644); err != nil {
		t.Fatal(err)
	}
	dashboard := `apiVersion: bffx.io/v1alpha1
kind: AdminDashboard
metadata:
  name: main
spec:
  widgets:
    - type: metric
      title: Users
      query:
        resource: User
        aggregate: count
`
	if err := os.WriteFile(filepath.Join(manifestDir, "main.yaml"), []byte(dashboard), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(reg.AdminDashboards) != 1 {
		t.Fatalf("AdminDashboards = %d, want 1", len(reg.AdminDashboards))
	}
	graph, err := BuildAdminGraph(reg)
	if err != nil {
		t.Fatalf("BuildAdminGraph: %v", err)
	}
	if graph.Dashboard == nil || len(graph.Dashboard.Widgets) != 1 {
		t.Fatalf("dashboard widgets = %v", graph.Dashboard)
	}
	if graph.Dashboard.Widgets[0].Title != "Users" {
		t.Fatalf("widget title = %q", graph.Dashboard.Widgets[0].Title)
	}
}

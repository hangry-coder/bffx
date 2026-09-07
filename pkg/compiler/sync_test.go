package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestSync(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Scaffold a project in the temp dir
	appName := "testapp"
	if err := generator.ScaffoldNewProject(tmp, appName, generator.ProjectOptions{}); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	appRoot := filepath.Join(tmp, appName)
	addLocalReplace(t, appRoot)

	// Run Sync
	if _, err := Sync(appRoot, false); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Check output (compiler emits graph + orchestrator registry, not legacy gen/go/*.gen.go).
	outFiles := []string{
		".bffx/graph.json",
		".bffx/admin-graph.json",
		".bffx/graph.hash",
		".bffx/build-profile.json",
		".bffx/diagnostics.json",
		".bffx/openapi.json",
		"cmd/api/registry.gen.go",
		".bffx/gen/types/bffx_types_helpers.gen.go",
		".bffx/gen/types/user_types.gen.go",
	}

	for _, f := range outFiles {
		p := filepath.Join(appRoot, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s does not exist", f)
		}
	}

	mainGo, err := os.ReadFile(filepath.Join(appRoot, "cmd/api/main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(mainGo), "ActionHandlers, HookHandlers") {
		t.Errorf("cmd/api/main.go must wire ActionHandlers and HookHandlers after sync; got:\n%s", mainGo)
	}
}

func TestSyncWithWire(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Scaffold a project in the temp dir
	appName := "testapp-wire"
	if err := generator.ScaffoldNewProject(tmp, appName, generator.ProjectOptions{}); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	appRoot := filepath.Join(tmp, appName)
	addLocalReplace(t, appRoot)

	// Update project.yaml to enable wire
	projectYamlPath := filepath.Join(appRoot, "bffx", "project.yaml")
	content, err := os.ReadFile(projectYamlPath)
	if err != nil {
		t.Fatalf("failed to read project.yaml: %v", err)
	}

	// Correctly insert the wire config inside spec.runtime
	newContent := strings.ReplaceAll(string(content), "    streaming:\n      enabled: false", "    streaming:\n      enabled: false\n    wire:\n      enabled: true\n      package: \"testapp.wire.v1\"")
	newContent = strings.ReplaceAll(newContent, "    streaming:\n      enabled: true", "    streaming:\n      enabled: true\n    wire:\n      enabled: true\n      package: \"testapp.wire.v1\"")

	if err := os.WriteFile(projectYamlPath, []byte(newContent), 0o644); err != nil {
		t.Fatalf("failed to write updated project.yaml: %v", err)
	}
	manifest.InvalidateLoadAllCache(appRoot)

	// Run Sync
	if _, err := Sync(appRoot, false); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Verify wire artifacts were created
	wireFiles := []string{
		".bffx/proto/bffx/v1/service.proto",
		".bffx/buf.yaml",
		".bffx/buf.gen.yaml",
	}

	for _, f := range wireFiles {
		p := filepath.Join(appRoot, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected wire file %s does not exist", f)
		}
	}

	// Read generated service.proto and verify package
	protoPath := filepath.Join(appRoot, ".bffx/proto/bffx/v1/service.proto")
	protoBytes, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("failed to read generated service.proto: %v", err)
	}

	protoText := string(protoBytes)
	if !strings.Contains(protoText, "package testapp.wire.v1;") {
		t.Errorf("expected package 'testapp.wire.v1' in service.proto, got:\n%s", protoText)
	}

	registryPath := filepath.Join(appRoot, "cmd/api/registry.gen.go")
	registryBytes, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read registry.gen.go: %v", err)
	}
	registry := string(registryBytes)
	if !strings.Contains(registry, "router.NewGRPCProxyHTTPRequest") {
		t.Errorf("registry.gen.go must use router.NewGRPCProxyHTTPRequest for gRPC proxies")
	}
	if !strings.Contains(registry, "MarshalProtoResourcePayload") {
		t.Errorf("registry.gen.go must marshal gRPC resource requests via router.MarshalProtoResourcePayload (manifest snake_case)")
	}
	if !strings.Contains(registry, "CreateResourceGRPC") {
		t.Errorf("registry.gen.go must create resources via router.CreateResourceGRPC")
	}
	if !strings.Contains(registry, "UpdateResourceGRPC") {
		t.Errorf("registry.gen.go must update resources via router.UpdateResourceGRPC")
	}
}

func addLocalReplace(t *testing.T, appRoot string) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	bffxRoot := filepath.Clean(filepath.Join(wd, "../.."))
	goModPath := filepath.Join(appRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	newContent := string(content) + "\nreplace github.com/hangry-coder/bffx => " + bffxRoot + "\n"
	if err := os.WriteFile(goModPath, []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

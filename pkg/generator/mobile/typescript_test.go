package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestGenerateTypeScriptClient(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ts-client-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "testapp"},
		},
	}

	resManifest := &manifest.Manifest{
		Kind:     "Resource",
		Metadata: manifest.Metadata{Name: "Post"},
	}
	resSpec := manifest.ResourceSpec{
		Fields: []manifest.ResourceField{
			{Name: "title", Type: "string", Required: true},
			{Name: "views", Type: "int"},
			{Name: "published", Type: "bool"},
		},
	}
	specBytes, _ := yaml.Marshal(resSpec)
	yaml.Unmarshal(specBytes, &resManifest.Spec)
	reg.Resources = append(reg.Resources, resManifest)

	err = GenerateTypeScriptClient(tempDir, reg)
	if err != nil {
		t.Fatalf("GenerateTypeScriptClient failed: %v", err)
	}

	genFile := filepath.Join(tempDir, "clients", "typescript", "src", "bffx.gen.ts")
	contentBytes, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	content := string(contentBytes)
	for _, expected := range []string{
		"export interface Post",
		"export class RecordService<T extends { id: string }>",
		"export class RealtimeService",
		"export class BFFXClient",
		"subscribe(callback: (event: RealtimeEvent<T>) => void): Unsubscribe",
		"get posts(): RecordService<Post>",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("expected generated TypeScript to contain %q", expected)
		}
	}
}

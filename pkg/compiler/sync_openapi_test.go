package compiler

import (
	"encoding/json"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestBuildOpenAPIIncludesHealthAndAuthPaths(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	raw, err := buildOpenAPI(reg)
	if err != nil {
		t.Fatalf("buildOpenAPI: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	if doc["openapi"] != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0, got %v", doc["openapi"])
	}

	paths, _ := doc["paths"].(map[string]any)
	for _, p := range []string{"/health", "/api/v1/auth/login", "/api/v1/realtime"} {
		if paths[p] == nil {
			t.Fatalf("expected path %q in openapi doc", p)
		}
	}

	// Verify tags
	tags, ok := doc["tags"].([]any)
	if !ok || len(tags) == 0 {
		t.Fatalf("expected tags to be populated, got %v", doc["tags"])
	}

	// Verify servers
	servers, ok := doc["servers"].([]any)
	if !ok || len(servers) == 0 {
		t.Fatalf("expected servers to be populated, got %v", doc["servers"])
	}
}

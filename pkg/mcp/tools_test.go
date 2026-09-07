package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestMCPServer_toolsCall_manifestRead_TraversalBlocked(t *testing.T) {
	tempDir := t.TempDir()
	s := NewServer(tempDir, &manifest.Registry{})

	// Create a safe file in the tempDir
	safePath := filepath.Join(tempDir, "project.yaml")
	if err := os.WriteFile(safePath, []byte("apiVersion: bffx.io/v1alpha1"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Safe relative path within project root should be read successfully
	paramsSafe, _ := json.Marshal(map[string]any{
		"name": "manifest.read",
		"arguments": map[string]any{
			"path": "project.yaml",
		},
	})
	respSafe := s.handleRequest(&Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: paramsSafe})
	if respSafe == nil || respSafe.Error != nil {
		t.Fatalf("expected successful response, got error: %+v", respSafe)
	}

	resMap, ok := respSafe.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", respSafe.Result)
	}
	contentList, _ := resMap["content"].([]map[string]any)
	if len(contentList) == 0 {
		t.Fatalf("expected content, got empty list")
	}
	cMap := contentList[0]
	if !strings.Contains(cMap["text"].(string), "apiVersion") {
		t.Errorf("unexpected content: %+v", resMap)
	}

	// 2. Traversal path outside project root should be blocked
	traversalPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		filepath.Join("..", filepath.Base(tempDir), ".."),
	}

	for _, badPath := range traversalPaths {
		paramsBad, _ := json.Marshal(map[string]any{
			"name": "manifest.read",
			"arguments": map[string]any{
				"path": badPath,
			},
		})
		respBad := s.handleRequest(&Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: paramsBad})
		if respBad == nil {
			t.Errorf("expected response, got nil for path: %s", badPath)
			continue
		}

		resBadMap, ok := respBad.Result.(map[string]any)
		if !ok {
			t.Errorf("expected map result, got %T for path: %s", respBad.Result, badPath)
			continue
		}

		if isErr, _ := resBadMap["isError"].(bool); !isErr {
			t.Errorf("expected isError to be true for path: %s, response: %+v", badPath, resBadMap)
		}

		errContent, _ := resBadMap["content"].([]map[string]any)
		if len(errContent) == 0 {
			t.Errorf("expected error content list, got empty/nil for path: %s", badPath)
			continue
		}
		ecMap := errContent[0]
		if !strings.Contains(ecMap["text"].(string), "access denied") {
			t.Errorf("expected 'access denied' in error text for path: %s, got: %+v", badPath, ecMap)
		}
	}
}

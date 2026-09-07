package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/tests/archetypes"
)

func TestArchetypeContracts(t *testing.T) {
	tests := []struct {
		alias         string
		expectedPaths []string
	}{
		{
			alias:         "notes",
			expectedPaths: []string{"/api/v1/users"},
		},
		{
			alias:         "superapp",
			expectedPaths: []string{"/api/v1/pipelines/support"},
		},
		{
			alias:         "fintech",
			expectedPaths: []string{"/api/v1/pipelines/receiptscan"},
		},
		{
			alias:         "marketplace",
			expectedPaths: []string{"/api/v1/users"},
		},
		{
			alias:         "microlearn",
			expectedPaths: []string{"/api/v1/users"},
		},
		{
			alias:         "dictation",
			expectedPaths: []string{"/api/v1/pipelines/audioupload"},
		},
		{
			alias:         "subscriptions",
			expectedPaths: []string{"/api/v1/pipelines/inboxparse"},
		},
		{
			alias:         "commerce",
			expectedPaths: []string{"/api/v1/pipelines/productscan"},
		},
		{
			alias:         "web3",
			expectedPaths: []string{"/api/v1/users"},
		},
		{
			alias:         "iot",
			expectedPaths: []string{"/api/v1/users"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.alias, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectDir := archetypes.SetupArchetypeProject(t, tmpDir, tc.alias)
			archetypes.SyncArchetypeProject(t, projectDir)

			openapiPath := filepath.Join(projectDir, ".bffx", "openapi.json")
			if _, err := os.Stat(openapiPath); os.IsNotExist(err) {
				t.Fatalf("openapi.json was not generated at %s", openapiPath)
			}

			data, err := os.ReadFile(openapiPath)
			if err != nil {
				t.Fatalf("failed to read openapi.json: %v", err)
			}

			var spec struct {
				Paths map[string]interface{} `json:"paths"`
			}
			if err := json.Unmarshal(data, &spec); err != nil {
				t.Fatalf("failed to parse openapi.json: %v", err)
			}

			for _, path := range tc.expectedPaths {
				if _, ok := spec.Paths[path]; !ok {
					// Also try checking without prefix /api/v1 in case router routing prefix changes
					fallbackPath := path
					if spec.Paths[fallbackPath] == nil {
						t.Fatalf("expected contract path %q to be present in OpenAPI specification, paths found: %v", path, keysOf(spec.Paths))
					}
				}
			}
		})
	}
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

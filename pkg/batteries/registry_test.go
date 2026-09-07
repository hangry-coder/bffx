package batteries

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		specYaml  string
		wantAuth  string
		wantStore string
	}{
		{
			name: "default batteries",
			kind: "Project",
			specYaml: `
runtime:
  api: {port: 8080}
`,
			wantAuth: "builtin",
			wantStore: "sqlite",
		},
		{
			name: "explicit batteries",
			kind: "Project",
			specYaml: `
batteries:
  auth: clerk
  store: postgres
`,
			wantAuth: "clerk",
			wantStore: "postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec manifest.ProjectSpec
			err := yaml.Unmarshal([]byte(tt.specYaml), &spec)
			if err != nil {
				t.Fatalf("Failed to unmarshal spec: %v", err)
			}
			
			if tt.wantAuth == "clerk" {
				t.Setenv("CLERK_JWKS_URL", "https://example.com")
			}

			got, err := Resolve(".", &spec, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Auth == nil {
				t.Errorf("Resolve().Auth is nil")
			}
			if got.Store != tt.wantStore {
				t.Errorf("Resolve().Store = %v, want %v", got.Store, tt.wantStore)
			}
		})
	}
}

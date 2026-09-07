package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoad(f *testing.F) {
	// Seed with valid examples
	f.Add([]byte(`
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: test
spec:
  runtime: { api: { language: go, port: 8080 } }
  store: { mode: memory }
`))

	f.Fuzz(func(t *testing.T, data []byte) {
		temp, err := os.MkdirTemp("", "bffx-fuzz-*")
		if err != nil {
			return
		}
		defer os.RemoveAll(temp)

		path := filepath.Join(temp, "test.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}

		// We don't care if it errors, only if it crashes/panics
		_, _ = Load(path)
	})
}

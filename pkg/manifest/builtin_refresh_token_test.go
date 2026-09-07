package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadAll_InjectsBuiltinRefreshTokenMinimalProject covers the Week 6 P3
// default-path without requiring `go test -tags=integration` (see also
// tests/integration for the tagged variant).
func TestLoadAll_InjectsBuiltinRefreshTokenMinimalProject(t *testing.T) {
	root := t.TempDir()
	bffxDir := filepath.Join(root, "bffx")
	require.NoError(t, os.MkdirAll(bffxDir, 0o755))

	project := `apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: TmpApp
spec:
  store:
    mode: memory
  app:
    namespace: tmp
    apiPrefix: /api/v1
`
	require.NoError(t, os.WriteFile(filepath.Join(bffxDir, "project.yaml"), []byte(project), 0o644))

	note := `apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: Note
spec:
  routes:
    crud: true
  policy:
    read: authenticated
    write: authenticated
  fields:
    - {name: body, type: string, required: true}
`
	require.NoError(t, os.WriteFile(filepath.Join(bffxDir, "note.yaml"), []byte(note), 0o644))

	reg, err := LoadAll(root)
	require.NoError(t, err)
	_, ok := reg.GetResource("RefreshToken")
	require.True(t, ok, "expected built-in RefreshToken resource")
}

//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"

	"github.com/stretchr/testify/require"
)

// TestLoadAll_InjectsRefreshToken verifies Week 6 default: projects without a
// RefreshToken resource get a built-in definition when refreshTokens is not disabled.
func TestLoadAll_InjectsRefreshToken(t *testing.T) {
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

	reg, err := manifest.LoadAll(root)
	require.NoError(t, err)
	_, ok := reg.GetResource("RefreshToken")
	require.True(t, ok, "expected built-in RefreshToken resource")
}

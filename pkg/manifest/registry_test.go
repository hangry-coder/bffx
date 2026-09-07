package manifest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRegistry_LoadAll(t *testing.T) {
	// Root should be the directory containing 'bffx'
	root, _ := filepath.Abs("testdata")
	reg, err := LoadAll(root)
	require.NoError(t, err)
	require.NotNil(t, reg)

	assert.Equal(t, "TestProject", reg.Project.Metadata.Name)

	// Post resource from file
	post, ok := reg.GetResource("Post")
	assert.True(t, ok)
	assert.NotNil(t, post)

	// User resource from internal defaults
	user, ok := reg.GetResource("User")
	assert.True(t, ok)
	assert.NotNil(t, user)
}

func TestRegistry_ResourceHash(t *testing.T) {
	root, _ := filepath.Abs("testdata")
	reg, err := LoadAll(root)
	require.NoError(t, err)

	h1 := reg.ResourceHash()
	assert.NotEmpty(t, h1)

	// Hash should be stable
	h2 := reg.ResourceHash()
	assert.Equal(t, h1, h2)
}

func TestRegistry_LoadAll_Failures(t *testing.T) {
	// 1. Missing directory
	_, err := LoadAll("/nonexistent/path/for/bffx")
	assert.Error(t, err)

	// 2. Malformed YAML
	root, _ := filepath.Abs("testdata/bad_yaml")
	_, err = LoadAll(root)
	assert.Error(t, err)
}

func TestRegistry_GetResource_NotFound(t *testing.T) {
	root, _ := filepath.Abs("testdata")
	reg, err := LoadAll(root)
	require.NoError(t, err)
	_, ok := reg.GetResource("NoSuchResourceForTestXYZ")
	assert.False(t, ok)
}

func TestResourceTree_BoolForm(t *testing.T) {
	var spec ResourceSpec
	err := yaml.Unmarshal([]byte(`
routes:
  crud: true
policy:
  read: public
  write: public
fields:
  - {name: body, type: string}
tree: true
`), &spec)
	require.NoError(t, err)
	assert.True(t, TreeEnabled(spec.Tree))
}

func TestResourceTree_StringForm_Legacy(t *testing.T) {
	var spec ResourceSpec
	err := yaml.Unmarshal([]byte(`
routes:
  crud: true
policy:
  read: public
  write: public
fields:
  - {name: body, type: string}
tree: ownership
`), &spec)
	require.NoError(t, err)
	assert.True(t, TreeEnabled(spec.Tree))
}

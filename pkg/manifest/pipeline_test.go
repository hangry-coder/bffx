package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPipelineSpec_Unmarshal(t *testing.T) {
	manifestYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: TestMealVision
spec:
  type: ingestion
  route:
    method: POST
    path: /api/v1/meals/scan
    auth: required
  model_routing:
    - gemini
    - openrouter
  catalog:
    adapter: openfoodfacts
  settings:
    compression_max_width: 800
    cache_bypass: true
`
	var m Manifest
	err := yaml.Unmarshal([]byte(manifestYaml), &m)
	require.NoError(t, err)
	assert.Equal(t, "Pipeline", m.Kind)
	assert.Equal(t, "TestMealVision", m.Metadata.Name)

	var spec PipelineSpec
	err = m.UnmarshalSpec(&spec)
	require.NoError(t, err)
	assert.Equal(t, "ingestion", spec.Type)
	assert.Equal(t, "POST", spec.Route.Method)
	assert.Equal(t, "/api/v1/meals/scan", spec.Route.Path)
	assert.Equal(t, "required", spec.Route.Auth)
	require.Len(t, spec.ModelRouting, 2)
	assert.Equal(t, "gemini", spec.ModelRouting[0])
	assert.Equal(t, "openrouter", spec.ModelRouting[1])
	require.NotNil(t, spec.Catalog)
	assert.Equal(t, "openfoodfacts", spec.Catalog.Adapter)
	assert.Equal(t, 800, spec.Settings.CompressionMaxWidth)
	assert.True(t, spec.Settings.CacheBypass)
}

func TestPipelineSpec_Validation(t *testing.T) {
	reg := &Registry{}

	// 1. Valid Pipeline
	m1 := &Manifest{
		Kind: "Pipeline",
		Spec: yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "type"}, {Kind: yaml.ScalarNode, Value: "ingestion"},
				{Kind: yaml.ScalarNode, Value: "model_routing"}, {Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "gemini"}}},
			},
		},
	}
	err := m1.Validate(reg)
	assert.NoError(t, err)

	// 2. Invalid Pipeline Type
	m2 := &Manifest{
		Kind: "Pipeline",
		Spec: yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "type"}, {Kind: yaml.ScalarNode, Value: "invalid_type"},
				{Kind: yaml.ScalarNode, Value: "model_routing"}, {Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "gemini"}}},
			},
		},
	}
	err = m2.Validate(reg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pipeline type")

	// 3. Missing Model Routing
	m3 := &Manifest{
		Kind: "Pipeline",
		Spec: yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "type"}, {Kind: yaml.ScalarNode, Value: "chatbot"},
			},
		},
	}
	err = m3.Validate(reg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline must define at least one entry in 'model_routing'")
}

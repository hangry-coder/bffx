package schema

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestFromRegistry(t *testing.T) {
	// Mock a resource spec
	specYaml := `
fields:
  - name: title
    type: string
    required: true
  - name: count
    type: int
  - name: price
    type: float
  - name: is_active
    type: bool
  - name: published_at
    type: date
  - name: metadata
    type: json
`
	var specNode yaml.Node
	if err := yaml.Unmarshal([]byte(specYaml), &specNode); err != nil {
		t.Fatalf("failed to unmarshal mock spec: %v", err)
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "Post"},
				Spec:     specNode,
			},
		},
	}

	tables := FromRegistry(reg)

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	table := tables[0]
	if table.Name != "post" {
		t.Errorf("expected table name 'post', got %q", table.Name)
	}

	expectedTypes := map[string]string{
		"id":           "text",
		"created_at":   "text",
		"updated_at":   "text",
		"created_by":   "text",
		"title":        "text",
		"count":        "bigint",
		"price":        "real",
		"is_active":    "boolean",
		"published_at": "timestamp",
		"metadata":     "jsonb",
	}

	for col, expType := range expectedTypes {
		def, ok := table.Columns[col]
		if !ok {
			t.Errorf("missing column %q", col)
			continue
		}
		if def.Type != expType {
			t.Errorf("column %q: expected type %q, got %q", col, expType, def.Type)
		}
	}
}

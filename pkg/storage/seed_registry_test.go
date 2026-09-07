package storage

import (
	"context"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestSeedRegistry_BlueprintDefaults(t *testing.T) {
	var spec yaml.Node
	if err := yaml.Unmarshal([]byte(`
resource: Note
count: 2
defaults:
  title: hello
`), &spec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Blueprints: []*manifest.Manifest{{
			Metadata: manifest.Metadata{Name: "SeedNotes"},
			Spec:     spec,
		}},
	}
	store := NewMemoryStore()
	if err := SeedRegistry(reg, store); err != nil {
		t.Fatalf("SeedRegistry: %v", err)
	}
	items, err := store.List(context.Background(), "Note", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(items))
	}
}

func TestSeedRegistry_UpsertByCode(t *testing.T) {
	var specInitial yaml.Node
	if err := yaml.Unmarshal([]byte(`
table: Widget
rows:
  - code: beta
    title: Beta Original
`), &specInitial); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Seeds: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Widgets"}, Spec: specInitial},
		},
	}
	store := NewMemoryStore()
	if err := SeedRegistry(reg, store); err != nil {
		t.Fatalf("SeedRegistry: %v", err)
	}

	var specUpsert yaml.Node
	if err := yaml.Unmarshal([]byte(`
table: Widget
order: 2
upsert: true
rows:
  - code: beta
    title: Beta Updated
`), &specUpsert); err != nil {
		t.Fatal(err)
	}
	reg.Seeds[0].Spec = specUpsert
	if err := SeedRegistry(reg, store, SeedOptions{Upsert: true}); err != nil {
		t.Fatalf("SeedRegistry upsert: %v", err)
	}
	updated, err := store.GetByField(context.Background(), "Widget", "code", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if updated["title"] != "Beta Updated" {
		t.Fatalf("expected updated title, got %v", updated["title"])
	}
}

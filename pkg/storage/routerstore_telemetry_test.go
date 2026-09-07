package storage

import (
	"context"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"

	"gopkg.in/yaml.v3"
)

func TestTelemetryRouting_ConsistentVerbs(t *testing.T) {
	primary := NewMemoryStore()
	telemetry := NewMemoryStore()

	var noteSpec, eventSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`
routes:
  crud: true
policy:
  read: public
  write: public
fields:
  - { name: title, type: string }
`), &noteSpec); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(`
telemetry: true
routes:
  crud: true
policy:
  read: public
  write: public
fields:
  - { name: title, type: string }
`), &eventSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "T"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Note"}, Spec: noteSpec},
			{Metadata: manifest.Metadata{Name: "Event"}, Spec: eventSpec},
		},
	}

	rs := &RouterStore{Primary: primary, Telemetry: telemetry, Registry: reg}
	ctx := context.Background()

	tests := []struct {
		name            string
		resource        string
		expectTelemetry bool
	}{
		{"non_telemetry_note", "Note", false},
		{"telemetry_event", "Event", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pick := primary
			other := telemetry
			if tc.expectTelemetry {
				pick, other = telemetry, primary
			}

			created, err := rs.Create(ctx, tc.resource, map[string]any{"title": "a"})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			id, _ := created["id"].(string)
			if id == "" {
				t.Fatal("missing id")
			}
			if n := countResource(pick, tc.resource); n != 1 {
				t.Fatalf("expected 1 row on target store, got %d", n)
			}
			if n := countResource(other, tc.resource); n != 0 {
				t.Fatalf("expected 0 rows on other store, got %d", n)
			}

			got, err := rs.Get(ctx, tc.resource, id)
			if err != nil || got["title"] != "a" {
				t.Fatalf("Get: err=%v got=%v", err, got)
			}

			list, err := rs.List(ctx, tc.resource, 10, 0)
			if err != nil || len(list) != 1 {
				t.Fatalf("List: err=%v len=%d", err, len(list))
			}

			upd, err := rs.Update(ctx, tc.resource, id, map[string]any{"title": "b"})
			if err != nil || upd["title"] != "b" {
				t.Fatalf("Update: err=%v upd=%v", err, upd)
			}

			if err := rs.Delete(ctx, tc.resource, id); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := rs.Get(ctx, tc.resource, id); err == nil {
				t.Fatal("expected not found after delete")
			}
			if n := countResource(pick, tc.resource); n != 0 {
				t.Fatalf("expected 0 rows after delete on target, got %d", n)
			}
		})
	}
}

func countResource(s *MemoryStore, resource string) int {
	ctx := context.Background()
	items, _ := s.List(ctx, resource, 0, 0)
	return len(items)
}

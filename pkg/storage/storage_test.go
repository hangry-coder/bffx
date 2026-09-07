package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"gopkg.in/yaml.v3"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()
	testStore(t, s)
}

func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	dbPath := "test.db"
	defer os.Remove(dbPath)

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}

	// Mock reconciliation with proper spec
	var specNode yaml.Node
	yaml.Unmarshal([]byte(`
fields:
  - { name: title, type: string, required: true }
  - { name: priority, type: int }
`), &specNode)

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "Task"},
				Spec:     specNode,
			},
		},
	}
	if _, err := s.Reconcile(ctx, reg); err != nil {
		t.Fatalf("failed to reconcile: %v", err)
	}

	testStore(t, s)
}

func testStore(t *testing.T, s Store) {
	ctx := context.Background()
	resource := "Task"

	// 1. Create
	item, err := s.Create(ctx, resource, map[string]any{
		"title":    "Buy Milk",
		"priority": 10,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if item == nil {
		t.Fatalf("Create returned nil")
	}
	if idVal, ok := item["id"]; !ok || idVal == "" {
		t.Fatalf("expected id to be set, got %+v", item)
	}
	id := item["id"].(string)

	// 2. Get
	found, err := s.Get(ctx, resource, id)
	if err != nil || found["title"] != "Buy Milk" {
		t.Errorf("failed to get item: %v", err)
	}

	// 3. Update
	updated, err := s.Update(ctx, resource, id, map[string]any{"priority": int64(20)})
	if err != nil {
		t.Fatalf("Update failed for id %s: %v", id, err)
	}
	if val := updated["priority"]; val != int64(20) {
		t.Errorf("expected priority 20, got %v (%T)", val, val)
	}

	// 4. List & Pagination
	s.Create(ctx, resource, map[string]any{"title": "Task 2"})
	s.Create(ctx, resource, map[string]any{"title": "Task 3"})

	items, _ := s.List(ctx, resource, 2, 0)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	itemsOffset, _ := s.List(ctx, resource, 1, 2)
	if len(itemsOffset) != 1 {
		t.Errorf("expected 1 item with offset, got %d", len(itemsOffset))
	}

	// 5. Delete
	if err := s.Delete(ctx, resource, id); err != nil {
		t.Errorf("failed to delete item: %v", err)
	}
	_, err = s.Get(ctx, resource, id)
	if err == nil {
		t.Errorf("item still exists after delete")
	}
}

func TestSQLInjectionPrevention(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_sqli.db"
	defer os.Remove(dbPath)

	s, _ := NewSQLiteStore(dbPath)
	malicious := "tasks; DROP TABLE users; --"

	// 1. Test CRUD methods with malicious resource name
	if items, _ := s.List(ctx, malicious, 10, 0); items != nil {
		t.Error("List should return nil for malicious resource name")
	}

	if _, err := s.Get(ctx, malicious, "1"); err == nil {
		t.Error("Get should return error for malicious resource name")
	}

	if item, err := s.Create(ctx, malicious, map[string]any{"foo": "bar"}); err == nil || item != nil {
		t.Error("Create should return error for malicious resource name")
	}

	if _, err := s.Update(ctx, malicious, "1", map[string]any{"foo": "bar"}); err == nil {
		t.Error("Update should return error for malicious resource name")
	}

	if err := s.Delete(ctx, malicious, "1"); err == nil {
		t.Error("Delete should return error for malicious resource name")
	}

	// 2. Test Reconcile with malicious table/column names
	var specNode yaml.Node
	yaml.Unmarshal([]byte(`
fields:
  - { name: "good", type: string }
  - { name: "bad; DROP TABLE users; --", type: string }
`), &specNode)

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "Malicious; DROP TABLE users; --"},
				Spec:     specNode,
			},
		},
	}

	// Reconcile should handle it gracefully (it logs and continues for individual resources/columns)
	if _, err := s.Reconcile(ctx, reg); err != nil {
		t.Fatalf("Reconcile failed catastrophically: %v", err)
	}
}

func TestDeepCopy(t *testing.T) {
	m := map[string]any{
		"name": "root",
		"nested": map[string]any{
			"key": "val",
		},
		"list": []any{
			"one",
			map[string]any{"two": 2},
			[]any{3, 4},
		},
	}

	cp := DeepCopy(m)

	// Verify values
	assert.Equal(t, m, cp)

	// Verify deep copy (modification of cp should not affect m)
	cp["name"] = "new"
	cp["nested"].(map[string]any)["key"] = "newval"
	cp["list"].([]any)[0] = "newone"
	cp["list"].([]any)[1].(map[string]any)["two"] = 22

	assert.Equal(t, "root", m["name"])
	assert.Equal(t, "val", m["nested"].(map[string]any)["key"])
	assert.Equal(t, "one", m["list"].([]any)[0])
	assert.Equal(t, 2, m["list"].([]any)[1].(map[string]any)["two"])
	
	// Nil case
	assert.Nil(t, DeepCopy(nil))
}

func TestValidateName(t *testing.T) {
	assert.NoError(t, validateName("Users"))
	assert.NoError(t, validateName("user_id_123"))
	assert.Error(t, validateName("user; DROP TABLE"))
	assert.Error(t, validateName("user--"))
}

func TestRouterStore(t *testing.T) {
	primary := NewMemoryStore()
	telemetry := NewMemoryStore()
	
	var spec yaml.Node
	yaml.Unmarshal([]byte(`telemetry: true`), &spec)
	
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "Log"},
				Spec:     spec,
			},
		},
	}
	
	rs := &RouterStore{
		Primary:   primary,
		Telemetry: telemetry,
		Registry:  reg,
	}
	
	ctx := context.Background()
	
	// 1. Non-telemetry resource -> Primary
	rs.Create(ctx, "Task", map[string]any{"title": "test"})
	list, _ := primary.List(ctx, "Task", 10, 0)
	assert.Len(t, list, 1)
	
	// 2. Telemetry resource -> Telemetry
	rs.Create(ctx, "Log", map[string]any{"msg": "event"})
	list, _ = telemetry.List(ctx, "Log", 10, 0)
	assert.Len(t, list, 1)
	
	// Verify it's NOT in primary
	list, _ = primary.List(ctx, "Log", 10, 0)
	assert.Len(t, list, 0)
}

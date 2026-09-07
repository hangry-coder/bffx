package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	var specNode yaml.Node
	err = yaml.Unmarshal([]byte(`
fields:
  - { name: title, type: string }
  - { name: priority, type: number }
  - { name: tags, type: array }
`), &specNode)
	if err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "Task"}, Spec: specNode},
		},
	}
	_, err = s.Reconcile(context.Background(), reg)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func TestSQLiteStore_CRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// 1. Create
	payload := map[string]any{
		"title":    "Fix tests",
		"priority": 1.0,
		"tags":     []any{"urgent", "dev"},
	}
	created, err := s.Create(ctx, "Task", payload)
	assert.NoError(t, err)
	assert.NotEmpty(t, created["id"])
	assert.Equal(t, "Fix tests", created["title"])
	id := created["id"].(string)

	// 2. Get
	fetched, err := s.Get(ctx, "Task", id)
	assert.NoError(t, err)
	assert.Equal(t, "Fix tests", fetched["title"])

	// 3. Update
	updatePayload := map[string]any{
		"title": "Fix tests updated",
	}
	updated, err := s.Update(ctx, "Task", id, updatePayload)
	assert.NoError(t, err)
	assert.Equal(t, "Fix tests updated", updated["title"])

	// 4. List
	list, err := s.List(ctx, "Task", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	// 5. Delete
	err = s.Delete(ctx, "Task", id)
	assert.NoError(t, err)
	
	// Verify deletion
	_, err = s.Get(ctx, "Task", id)
	assert.Error(t, err)
}

func TestSQLiteStore_QueryBuilder(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.Create(ctx, "Task", map[string]any{"title": "A", "priority": 1.0})
	s.Create(ctx, "Task", map[string]any{"title": "B", "priority": 2.0})
	s.Create(ctx, "Task", map[string]any{"title": "C", "priority": 3.0})

	// Where
	res, err := s.Query(ctx, "Task").Where("priority", ">", 1.5).OrderBy("priority", true).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, "C", res[0]["title"]) // Descending order
	
	// WhereIn
	res, err = s.Query(ctx, "Task").WhereIn("title", []any{"A", "C"}).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 2)

	// Limit Offset
	res, err = s.Query(ctx, "Task").OrderBy("priority", false).Limit(1).Offset(1).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "B", res[0]["title"])
}

func TestSQLiteStore_ListByOwner_And_GetByField(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.Create(ctx, "Task", map[string]any{"title": "A", "created_by": "user1"})
	s.Create(ctx, "Task", map[string]any{"title": "B", "created_by": "user1"})
	s.Create(ctx, "Task", map[string]any{"title": "C", "created_by": "user2"})

	// ListByOwner
	res, err := s.ListByOwner(ctx, "Task", "user1", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, res, 2)

	// GetByField
	item, err := s.GetByField(ctx, "Task", "title", "C")
	assert.NoError(t, err)
	assert.Equal(t, "C", item["title"])
}

func TestSQLiteStore_TreeHelpers(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// They just return not implemented usually for sqlite unless tree enabled
	_, err := s.GetChildren(ctx, "Task", "1")
	assert.NoError(t, err)

	_, err = s.GetAncestors(ctx, "Task", "1")
	assert.NoError(t, err)
}

func TestSQLiteStore_ColumnValidation(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// 1. Create with invalid column key
	payload := map[string]any{
		"title; DROP TABLE Task; --": "injection",
	}
	_, err := s.Create(ctx, "Task", payload)
	assert.Error(t, err, "expected error when creating with malicious column name")

	// 2. Update with invalid column key
	// first create a valid record
	validPayload := map[string]any{
		"title": "valid",
	}
	created, err := s.Create(ctx, "Task", validPayload)
	assert.NoError(t, err)
	id := created["id"].(string)

	updatePayload := map[string]any{
		"title; DROP TABLE Task; --": "injection",
	}
	_, err = s.Update(ctx, "Task", id, updatePayload)
	assert.Error(t, err, "expected error when updating with malicious column name")
}

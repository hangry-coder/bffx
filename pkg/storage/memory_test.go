package storage

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMemoryStore_GetByField(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	// Create a record
	payload := map[string]any{"email": "test@example.com", "name": "Test"}
	created, err := s.Create(ctx, "User", payload)
	assert.NoError(t, err)
	assert.NotNil(t, created)

	// Get it back
	found, err := s.GetByField(ctx, "User", "email", "test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", found["email"])
	assert.Equal(t, "Test", found["name"])

	// Missing field
	_, err = s.GetByField(ctx, "User", "email", "none@example.com")
	assert.Error(t, err)

	// Missing resource
	_, err = s.GetByField(ctx, "None", "email", "any")
	assert.Error(t, err)
}

func TestMemoryStore_QueryMatrix(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Create(ctx, "Item", map[string]any{"id": "1", "val": 10, "tag": "apple"})
	s.Create(ctx, "Item", map[string]any{"id": "2", "val": 20, "tag": "banana"})
	s.Create(ctx, "Item", map[string]any{"id": "3", "val": 30, "tag": "apricot"})

	// 1. Comparison operators
	res, _ := s.Query(ctx, "Item").Where("val", ">", "15").Execute(ctx)
	assert.Equal(t, 2, len(res), "should find 2 items with val > 15")

	res, _ = s.Query(ctx, "Item").Where("val", "<=", "20").Execute(ctx)
	assert.Equal(t, 2, len(res), "should find 2 items with val <= 20")

	res, _ = s.Query(ctx, "Item").Where("val", ">=", "20").Execute(ctx)
	assert.Equal(t, 2, len(res), "should find 2 items with val >= 20")

	res, _ = s.Query(ctx, "Item").OrderBy("val", true).Where("val", "lte", "25").Execute(ctx)
	assert.Equal(t, 2, len(res), "lte alias should match val <= 25")

	res, _ = s.Query(ctx, "Item").Where("val", "!=", "20").Execute(ctx)
	assert.Equal(t, 2, len(res), "should find 2 items with val != 20")

	// 2. Like operator
	res, _ = s.Query(ctx, "Item").Where("tag", "like", "ap%").Execute(ctx)
	assert.Equal(t, 2, len(res), "should find apple and apricot")

	// 3. WhereIn
	res, _ = s.Query(ctx, "Item").WhereIn("id", []any{"1", "3"}).Execute(ctx)
	assert.Equal(t, 2, len(res), "WhereIn should find 2 items")

	// 4. Empty WhereIn
	res, _ = s.Query(ctx, "Item").WhereIn("id", []any{}).Execute(ctx)
	assert.Equal(t, 0, len(res), "Empty WhereIn should find 0 items")

	// 5. Pagination
	// Note: MemoryStore doesn't guarantee order, but with 3 items, Limit(1) should return 1.
	res, _ = s.Query(ctx, "Item").Limit(1).Execute(ctx)
	assert.Equal(t, 1, len(res))

	res, _ = s.Query(ctx, "Item").Offset(10).Execute(ctx)
	assert.Equal(t, 0, len(res), "Offset beyond count should return empty")
}

func TestMemoryStore_QueryComprehensive(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	// Seed records
	s.Create(ctx, "User", map[string]any{"id": "1", "age": 20, "name": "Alice"})
	s.Create(ctx, "User", map[string]any{"id": "2", "age": 30, "name": "Bob"})
	s.Create(ctx, "User", map[string]any{"id": "3", "age": 40, "name": "Charlie"})

	// 1. OrderBy Ascending
	res, err := s.Query(ctx, "User").OrderBy("age", false).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Equal(t, "Alice", res[0]["name"])
	assert.Equal(t, "Bob", res[1]["name"])
	assert.Equal(t, "Charlie", res[2]["name"])

	// 2. OrderBy Descending
	res, err = s.Query(ctx, "User").OrderBy("age", true).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Equal(t, "Charlie", res[0]["name"])
	assert.Equal(t, "Bob", res[1]["name"])
	assert.Equal(t, "Alice", res[2]["name"])

	// 3. Count with operators
	// gt / >
	cnt, err := s.Query(ctx, "User").Where("age", "gt", 25).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// lt / <
	cnt, err = s.Query(ctx, "User").Where("age", "lt", 35).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// gte / >=
	cnt, err = s.Query(ctx, "User").Where("age", "gte", 30).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// lte / <=
	cnt, err = s.Query(ctx, "User").Where("age", "lte", 30).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// neq / !=
	cnt, err = s.Query(ctx, "User").Where("age", "neq", 30).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// eq / =
	cnt, err = s.Query(ctx, "User").Where("age", "eq", 30).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)

	// in
	cnt, err = s.Query(ctx, "User").Where("id", "in", []any{"1", "3"}).Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// like
	cnt, err = s.Query(ctx, "User").Where("name", "like", "Al%").Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)

	// 4. Invalid operator branches
	// WhereIn with non-slice in Where
	res, err = s.Query(ctx, "User").Where("id", "in", "not-a-slice").Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 0)

	cnt, err = s.Query(ctx, "User").Where("id", "in", "not-a-slice").Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, cnt)

	// 5. Offset edge cases
	res, err = s.Query(ctx, "User").Offset(3).Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}


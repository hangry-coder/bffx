package seeding

import (
	"context"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEngine_Seed(t *testing.T) {
	store := storage.NewMemoryStore()
	engine := NewEngine(store)

	template := map[string]any{
		"full_name": "faker:name",
		"email":     "faker:email",
		"age":       25,
	}

	err := engine.Seed(context.Background(), "users", 5, template)
	assert.NoError(t, err)

	res, err := store.List(context.Background(), "users", 10, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(res))

	for _, user := range res {
		assert.NotEmpty(t, user["full_name"])
		assert.NotEmpty(t, user["email"])
		assert.Equal(t, 25, user["age"]) // MemoryStore preserves types
	}
}

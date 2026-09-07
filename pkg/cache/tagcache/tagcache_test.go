package tagcache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisTagCache_CRUD(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	cache := NewRedisTagCache(client)
	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-key"
		body := []byte("hello world")
		tags := []string{"resource:User", "id:123"}
		
		err := cache.Set(ctx, key, body, tags, time.Minute)
		assert.NoError(t, err)

		val, err := cache.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, body, val)
	})

	t.Run("Get MISS", func(t *testing.T) {
		val, err := cache.Get(ctx, "non-existent")
		assert.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("Invalidate by Tag", func(t *testing.T) {
		key1 := "key1"
		key2 := "key2"
		tags := []string{"resource:Note"}

		cache.Set(ctx, key1, []byte("val1"), tags, time.Minute)
		cache.Set(ctx, key2, []byte("val2"), tags, time.Minute)

		deleted, err := cache.InvalidateTags(ctx, tags)
		assert.NoError(t, err)
		assert.Equal(t, 2, deleted)

		val1, _ := cache.Get(ctx, key1)
		assert.Nil(t, val1)
		val2, _ := cache.Get(ctx, key2)
		assert.Nil(t, val2)
	})

	t.Run("Partial Tag Invalidation", func(t *testing.T) {
		key1 := "key1"
		tags1 := []string{"tagA", "tagB"}
		cache.Set(ctx, key1, []byte("val1"), tags1, time.Minute)

		key2 := "key2"
		tags2 := []string{"tagB", "tagC"}
		cache.Set(ctx, key2, []byte("val2"), tags2, time.Minute)

		// Invalidate tagA -> should only kill key1
		deleted, _ := cache.InvalidateTags(ctx, []string{"tagA"})
		assert.Equal(t, 1, deleted)

		v1, _ := cache.Get(ctx, key1)
		assert.Nil(t, v1)
		v2, _ := cache.Get(ctx, key2)
		assert.NotNil(t, v2)
	})
}

func TestRedisTagCache_FailOpen(t *testing.T) {
	mr, _ := miniredis.Run()
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	cache := NewRedisTagCache(client)
	ctx := context.Background()

	// Kill redis
	mr.Close()

	t.Run("Get fails open", func(t *testing.T) {
		val, err := cache.Get(ctx, "any")
		assert.NoError(t, err) // Should not error
		assert.Nil(t, val)     // Should be MISS
	})

	t.Run("Set returns error (fail-degraded)", func(t *testing.T) {
		err := cache.Set(ctx, "any", []byte("data"), []string{"tag"}, time.Minute)
		assert.Error(t, err)
	})
}

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIndexedProvider(t *testing.T) {
	ctx := context.Background()
	local := NewLocalCache()
	ip := NewIndexedProvider(local)

	// 1. Set with tags
	err := ip.SetWithTags(ctx, "key1", []byte("val1"), []string{"tag1"}, 5*time.Second)
	assert.NoError(t, err)

	err = ip.SetWithTags(ctx, "key2", []byte("val2"), []string{"tag1", "tag2"}, 5*time.Second)
	assert.NoError(t, err)

	// Verify key1 and key2 are retrievable
	v1, err := ip.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("val1"), v1)

	v2, err := ip.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, []byte("val2"), v2)

	// Verify cache index key exists and contains keys
	indexVal, err := local.Get(ctx, "cache_index:tag1")
	assert.NoError(t, err)
	assert.Contains(t, string(indexVal), "key1")
	assert.Contains(t, string(indexVal), "key2")

	// 2. Invalidate tag2
	deleted, err := ip.InvalidateTags(ctx, []string{"tag2"})
	assert.NoError(t, err)
	assert.True(t, deleted >= 1)

	// key2 should be gone, key1 should remain
	v1, err = ip.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("val1"), v1)

	v2, err = ip.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Nil(t, v2)

	// 3. Invalidate tag1
	deleted, err = ip.InvalidateTags(ctx, []string{"tag1"})
	assert.NoError(t, err)
	assert.True(t, deleted >= 1)

	// key1 should be gone now too
	v1, err = ip.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, v1)
}

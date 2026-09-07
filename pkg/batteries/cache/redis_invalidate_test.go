package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	bffxcache "github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
)

func TestRedisProvider_InvalidatePattern_UsesKeyPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	p := NewRedisProvider(rdb)
	ctx := context.Background()

	logicalKey := middleware.ActionCacheKeyPrefix + "user1:abc"
	fullKey := bffxcache.RedisKeyPrefix + logicalKey
	if err := rdb.Set(ctx, fullKey, `{"ok":true}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "bffx:other:1", "z", 0).Err(); err != nil {
		t.Fatal(err)
	}

	n, err := p.InvalidatePattern(ctx, middleware.ActionCacheInvalidatePattern)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
	exists, _ := rdb.Exists(ctx, fullKey).Result()
	if exists != 0 {
		t.Fatal("expected action cache key removed")
	}
	exists, _ = rdb.Exists(ctx, "bffx:other:1").Result()
	if exists != 1 {
		t.Fatal("expected unrelated key to remain")
	}
}

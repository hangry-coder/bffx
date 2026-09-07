package middleware

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	cachebattery "github.com/hangry-coder/bffx/pkg/batteries/cache"
)

func TestInvalidatePattern_ClearsMatchingKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	
	p := cachebattery.NewRedisProvider(rdb)

	ctx := context.Background()
	if err := rdb.Set(ctx, "bffx:cache:k:bffx:action-cache:u1:aaa", `{"x":1}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "bffx:cache:k:bffx:action-cache:u2:bbb", `{"x":2}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "bffx:other:1", "z", 0).Err(); err != nil {
		t.Fatal(err)
	}
	// Tag-backed screen cache keys must not match ActionCacheInvalidatePattern (CRUD uses this glob only).
	if err := rdb.Set(ctx, "bffx:cache:k:screen1", `payload`, 0).Err(); err != nil {
		t.Fatal(err)
	}

	nDel, err := p.InvalidatePattern(ctx, ActionCacheInvalidatePattern)
	if err != nil {
		t.Fatal(err)
	}
	if nDel != 2 {
		t.Fatalf("expected 2 action-cache keys deleted, got %d", nDel)
	}

	n, err := rdb.DBSize(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 keys left (bffx:other:1 + bffx:cache:k:screen1), got %d", n)
	}
}

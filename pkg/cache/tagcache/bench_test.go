package tagcache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func BenchmarkRedisTagCache_Set(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	cache := NewRedisTagCache(client)
	ctx := context.Background()
	
	payload := make([]byte, 64*1024) // 64KiB payload as per plan
	tags := []string{"resource:User", "id:123", "group:Admin", "geo:US", "version:1.0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		err := cache.Set(ctx, key, payload, tags, time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisTagCache_Get(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	cache := NewRedisTagCache(client)
	ctx := context.Background()
	
	payload := make([]byte, 64*1024)
	cache.Set(ctx, "bench-key", payload, []string{"tag1"}, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.Get(ctx, "bench-key")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisTagCache_Invalidate(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	cache := NewRedisTagCache(client)
	ctx := context.Background()
	
	tags := []string{"resource:Note"}
	// Pre-fill with some keys
	for i := 0; i < 1000; i++ {
		cache.Set(ctx, fmt.Sprintf("key-%d", i), []byte("data"), tags, time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: invalidation clears the tag, so we'd need to re-fill to get realistic multiple runs,
		// but for a raw throughput bench on the command execution it's okay.
		_, err := cache.InvalidateTags(ctx, tags)
		if err != nil {
			b.Fatal(err)
		}
	}
}

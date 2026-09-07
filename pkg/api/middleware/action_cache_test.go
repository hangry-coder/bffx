package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	cachebattery "github.com/hangry-coder/bffx/pkg/batteries/cache"
)

func TestInvalidateActionCacheOnMutation_ClearsCacheAfterPOST(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	p := cachebattery.NewRedisProvider(rdb)

	ctx := context.Background()
	if err := p.Set(ctx, ActionCacheKeyPrefix+"u1:abc", []byte(`{"stale":true}`), 0); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/items", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	h := InvalidateActionCacheOnMutation(p)(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d", rr.Code)
	}
	if _, err := p.Get(ctx, ActionCacheKeyPrefix+"u1:abc"); err != nil {
		t.Fatal(err)
	}
	if v, _ := p.Get(ctx, ActionCacheKeyPrefix+"u1:abc"); v != nil {
		t.Fatal("expected action cache cleared after POST")
	}
}

func TestInvalidateActionCacheOnMutation_SkipsGET(t *testing.T) {
	p := cachebattery.NewMemoryProvider()
	ctx := context.Background()
	if err := p.Set(ctx, ActionCacheKeyPrefix+"u1:abc", []byte(`{"ok":true}`), 0); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/items", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := InvalidateActionCacheOnMutation(p)(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if v, _ := p.Get(ctx, ActionCacheKeyPrefix+"u1:abc"); v == nil {
		t.Fatal("GET should not bust action cache")
	}
}

func TestCacheKeyPrefixAndInvalidationPattern(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	p := cachebattery.NewRedisProvider(rdb)
	ctx := context.Background()

	// 1. Generate keys
	actionKey1 := ActionCacheKeyPrefix + "u1:something"
	actionKey2 := ActionCacheKeyPrefix + "anon:otherthing"
	nonActionKey := "other:u1:something"

	p.Set(ctx, actionKey1, []byte("val1"), 0)
	p.Set(ctx, actionKey2, []byte("val2"), 0)
	p.Set(ctx, nonActionKey, []byte("val3"), 0)

	// 2. Run invalidation pattern
	InvalidateActionCache(ctx, p)

	// 3. Verify
	if v, _ := p.Get(ctx, actionKey1); v != nil {
		t.Errorf("expected action cache key %q to be invalidated", actionKey1)
	}
	if v, _ := p.Get(ctx, actionKey2); v != nil {
		t.Errorf("expected action cache key %q to be invalidated", actionKey2)
	}
	if v, _ := p.Get(ctx, nonActionKey); v == nil {
		t.Errorf("expected unrelated key %q to remain in cache", nonActionKey)
	}
}

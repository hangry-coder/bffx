package cache

import (
	"net/http"
	"net/url"
	"testing"
)

func TestBuildHTTPScopedKey_Deterministic(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Device-ID", "d1")
	q := url.Values{"b": []string{"2"}, "a": []string{"1"}}

	k1 := BuildHTTPScopedKey(ActionResponseKeyPrefix, "", http.MethodGet, "/api/v1/items", q, []string{"X-Device-ID"}, headers.Get)
	k2 := BuildHTTPScopedKey(ActionResponseKeyPrefix, "", http.MethodGet, "/api/v1/items", q, []string{"X-Device-ID"}, headers.Get)
	if k1 != k2 {
		t.Fatalf("expected deterministic keys, got %q and %q", k1, k2)
	}
	expectedPrefix := ActionResponseKeyPrefix + "anon:"
	if len(k1) < len(expectedPrefix) || k1[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected anon-prefixed action key, got %q", k1)
	}
}

func TestBuildHTTPScopedKey_VaryChangesHash(t *testing.T) {
	headersA := http.Header{}
	headersA.Set("X-Device-ID", "d1")
	headersB := http.Header{}
	headersB.Set("X-Device-ID", "d2")
	q := url.Values{}

	k1 := BuildHTTPScopedKey(TaggedResponseKeyPrefix, "u1", http.MethodGet, "/api/v1/home", q, []string{"X-Device-ID"}, headersA.Get)
	k2 := BuildHTTPScopedKey(TaggedResponseKeyPrefix, "u1", http.MethodGet, "/api/v1/home", q, []string{"X-Device-ID"}, headersB.Get)
	if k1 == k2 {
		t.Fatalf("expected vary header change to alter key, got identical %q", k1)
	}
}

package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// NormalizeOwner maps an empty owner to the canonical anonymous bucket.
func NormalizeOwner(owner string) string {
	if strings.TrimSpace(owner) == "" {
		return "anon"
	}
	return owner
}

// BuildHTTPScopedKey returns a deterministic logical key for HTTP response caching.
// The key format is: <prefix><owner>:<sha256(method,path,query,vary headers)>.
func BuildHTTPScopedKey(prefix, owner, method, path string, query url.Values, varyHeaders []string, headerValue func(string) string) string {
	h := sha256.New()
	h.Write([]byte(strings.ToUpper(method)))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(query.Encode()))
	for _, name := range varyHeaders {
		h.Write([]byte{0})
		h.Write([]byte(strings.ToLower(name)))
		h.Write([]byte{':'})
		h.Write([]byte(headerValue(name)))
	}
	return prefix + NormalizeOwner(owner) + ":" + hex.EncodeToString(h.Sum(nil))
}

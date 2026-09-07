package router

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func resolvePath(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			return nil
		}
	}
	return current
}

func setNested(m map[string]any, path string, val any) {
	parts := strings.Split(path, ".")
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		if _, ok := m[key]; !ok {
			m[key] = make(map[string]any)
		}
		m = m[key].(map[string]any)
	}
	m[parts[len(parts)-1]] = val
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, true
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid json")
		return nil, false
	}
	return payload, true
}

func writeJSONWithETag(w http.ResponseWriter, r *http.Request, status int, payload any) {
	body, _ := json.Marshal(payload)
	etag := generateETag(body)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(status)
	w.Write(body)
}

func generateETag(body []byte) string {
	h := sha256.New()
	h.Write(body)
	return `W/"` + hex.EncodeToString(h.Sum(nil)) + `"`
}

package middleware

import (
	"net/http"
)

type claimsKey struct{}
type requestIDKey struct{}
type ipKey struct{}
type userAgentKey struct{}
type traceIDKey struct{}
type deviceIDKey struct{}

// ResponseCapture buffers the downstream response so we can mirror it into
// a cache after the handler returns.
type ResponseCapture struct {
	http.ResponseWriter
	Status int
	Body   []byte
}

func (r *ResponseCapture) WriteHeader(status int) {
	if r.Status == 0 {
		r.Status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *ResponseCapture) Write(b []byte) (int, error) {
	if r.Status == 0 {
		r.Status = http.StatusOK
	}
	r.Body = append(r.Body, b...)
	return r.ResponseWriter.Write(b)
}

// ValidDeviceID returns whether id is acceptable for X-Device-ID (length + charset).
func ValidDeviceID(id string) bool {
	n := len(id)
	if n < 8 || n > 128 {
		return false
	}
	for i := 0; i < n; i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return true
}

// SafePrefixRunes truncates s to at most max runes (used for guest display names).
func SafePrefixRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

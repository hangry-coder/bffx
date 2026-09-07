package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
)

// SecretsEqual compares two secret strings in constant time without leaking length
// differences via digest equality (SHA-256 of each operand).
func SecretsEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

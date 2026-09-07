package auth

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const RefreshTokenWirePrefix = "rt1."

// MintRefreshTokenCredential creates a row id, bcrypt hash (stored in RefreshToken.token),
// and the opaque wire value returned to clients: rt1.{id}.{secret}.
func MintRefreshTokenCredential() (rowID, wireToken, tokenHash string, err error) {
	rowID = uuid.New().String()
	secret := uuid.New().String()
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcryptCost())
	if err != nil {
		return "", "", "", fmt.Errorf("refresh token hash: %w", err)
	}
	wireToken = RefreshTokenWirePrefix + rowID + "." + secret
	return rowID, wireToken, string(hash), nil
}

// ParseRefreshWire splits rt1.{id}.{secret} from the client-presented token.
func ParseRefreshWire(presented string) (rowID, secret string, ok bool) {
	if !strings.HasPrefix(presented, RefreshTokenWirePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(presented, RefreshTokenWirePrefix)
	dot := strings.Index(rest, ".")
	if dot <= 0 {
		return "", "", false
	}
	rowID = rest[:dot]
	secret = rest[dot+1:]
	if rowID == "" || secret == "" {
		return "", "", false
	}
	return rowID, secret, true
}

// VerifyRefreshSecret compares a bcrypt hash with the secret portion of the wire token.
func VerifyRefreshSecret(storedHash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(secret)) == nil
}

package authbattery

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClerkProvider_ProductionRejectsHMACFallback(t *testing.T) {
	t.Setenv("BFFX_ENV", "production")
	t.Setenv("BFFX_CLERK_ALLOW_HMAC_FALLBACK", "")
	t.Setenv("BFFX_JWT_SECRET", "bffx-dev-jwt-secret-key-must-be-at-least-32-characters")

	p := &ClerkProvider{issuer: "https://clerk.example"}
	// Invalid for JWKS path — would have fallen back to HMAC in dev.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(os.Getenv("BFFX_JWT_SECRET")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ValidateToken(context.Background(), signed)
	if err == nil {
		t.Fatal("expected production to reject HMAC fallback without Clerk JWKS")
	}
}

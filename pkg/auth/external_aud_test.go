package auth

import (
	"context"
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyExternalAudience_Configured(t *testing.T) {
	t.Setenv("BFFX_GOOGLE_CLIENT_ID", "my-client-id.apps.googleusercontent.com")
	claims := jwt.MapClaims{"aud": "my-client-id.apps.googleusercontent.com"}
	if err := verifyExternalAudience("google", claims); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
	claims["aud"] = "wrong-aud"
	if err := verifyExternalAudience("google", claims); err == nil {
		t.Fatal("expected audience mismatch")
	}
	claims["aud"] = []any{"my-client-id.apps.googleusercontent.com"}
	if err := verifyExternalAudience("google", claims); err != nil {
		t.Fatalf("expected array aud match, got %v", err)
	}
}

func TestVerifyExternalAudience_SkipsWhenUnset(t *testing.T) {
	os.Unsetenv("BFFX_GOOGLE_CLIENT_ID")
	claims := jwt.MapClaims{"aud": "anything"}
	if err := verifyExternalAudience("google", claims); err != nil {
		t.Fatalf("expected skip when env unset, got %v", err)
	}
}

func TestValidateIdentityToken_MockStillWorks(t *testing.T) {
	p := GetExternalProvider()
	_, err := p.ValidateIdentityToken(context.Background(), "google", "MOCK_TOKEN_google_abc_Test")
	if err != nil {
		t.Fatalf("mock token: %v", err)
	}
}

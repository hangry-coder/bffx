package auth

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestExternalProvider_ValidateIdentityToken_Mock(t *testing.T) {
	p := GetExternalProvider()
	ctx := context.Background()

	t.Run("Valid mock google token", func(t *testing.T) {
		token := "MOCK_TOKEN_google_12345_JohnDoe"
		identity, err := p.ValidateIdentityToken(ctx, "google", token)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if identity.Sub != "12345" {
			t.Errorf("expected sub 12345, got %s", identity.Sub)
		}
		if identity.Email != "12345@example.com" {
			t.Errorf("expected email 12345@example.com, got %s", identity.Email)
		}
		if identity.Name != "JohnDoe" {
			t.Errorf("expected name JohnDoe, got %s", identity.Name)
		}
	})

	t.Run("Unsupported provider", func(t *testing.T) {
		token := "real_or_invalid_token"
		_, err := p.ValidateIdentityToken(ctx, "facebook", token)
		if err == nil {
			t.Fatal("expected error for unsupported provider, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported identity provider") {
			t.Errorf("expected unsupported provider message, got: %v", err)
		}
	})

	t.Run("Invalid parse formats", func(t *testing.T) {
		_, err := p.ValidateIdentityToken(ctx, "google", "invalid_jwt_format")
		if err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})

	t.Run("Mock token rejected in production", func(t *testing.T) {
		t.Setenv("BFFX_ENV", "production")
		_, err := p.ValidateIdentityToken(ctx, "google", "MOCK_TOKEN_google_12345_JohnDoe")
		if err == nil {
			t.Fatal("expected error in production, got nil")
		}
		if !strings.Contains(err.Error(), "not allowed in production") {
			t.Fatalf("unexpected error: %v", err)
		}
		os.Unsetenv("BFFX_ENV")
	})
}

package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ExternalProvider manages JWKS keys for Google and Apple OIDC.
type ExternalProvider struct {
	googleKf keyfunc.Keyfunc
	appleKf  keyfunc.Keyfunc
	mu       sync.RWMutex
}

var (
	extInstance *ExternalProvider
	extOnce     sync.Once
)

// GetExternalProvider returns the singleton instance of ExternalProvider.
func GetExternalProvider() *ExternalProvider {
	extOnce.Do(func() {
		extInstance = &ExternalProvider{}
	})
	return extInstance
}

func (p *ExternalProvider) initGoogle() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.googleKf != nil {
		return nil
	}
	kf, err := keyfunc.NewDefault([]string{"https://www.googleapis.com/oauth2/v3/certs"})
	if err != nil {
		return err
	}
	p.googleKf = kf
	return nil
}

func (p *ExternalProvider) initApple() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.appleKf != nil {
		return nil
	}
	kf, err := keyfunc.NewDefault([]string{"https://appleid.apple.com/auth/keys"})
	if err != nil {
		return err
	}
	p.appleKf = kf
	return nil
}

// ExternalIdentity represents the identity claims extracted from a verified ID token.
type ExternalIdentity struct {
	Sub   string
	Email string
	Name  string
}

// ValidateIdentityToken verifies the signature and claims of a Google or Apple ID token.
func (p *ExternalProvider) ValidateIdentityToken(ctx context.Context, provider string, tokenString string) (*ExternalIdentity, error) {
	// Sandbox/Mock validation for non-production/testing
	if strings.HasPrefix(tokenString, "MOCK_TOKEN_") {
		if os.Getenv("BFFX_ENV") == "production" {
			return nil, errors.New("mock identity tokens are not allowed in production")
		}
		parts := strings.Split(tokenString, "_")
		sub := "mock_sub"
		email := "mock@example.com"
		name := "Mock User"
		if len(parts) > 3 {
			sub = parts[3]
			email = sub + "@example.com"
			if len(parts) > 4 {
				name = parts[4]
			}
		}
		return &ExternalIdentity{
			Sub:   sub,
			Email: email,
			Name:  name,
		}, nil
	}

	var kf keyfunc.Keyfunc
	var expectedIssuers []string

	switch strings.ToLower(provider) {
	case "google":
		if err := p.initGoogle(); err != nil {
			return nil, fmt.Errorf("failed to init google keys: %w", err)
		}
		kf = p.googleKf
		expectedIssuers = []string{"https://accounts.google.com", "accounts.google.com"}
	case "apple":
		if err := p.initApple(); err != nil {
			return nil, fmt.Errorf("failed to init apple keys: %w", err)
		}
		kf = p.appleKf
		expectedIssuers = []string{"https://appleid.apple.com"}
	default:
		return nil, fmt.Errorf("unsupported identity provider: %s", provider)
	}

	token, err := jwt.Parse(tokenString, kf.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid identity token signature")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Verify Issuer
	iss, _ := claims["iss"].(string)
	issValid := false
	for _, expected := range expectedIssuers {
		if iss == expected {
			issValid = true
			break
		}
	}
	if !issValid {
		return nil, fmt.Errorf("invalid issuer: %s", iss)
	}

	// Verify Expiration
	if expVal, ok := claims["exp"]; ok {
		var exp int64
		switch v := expVal.(type) {
		case float64:
			exp = int64(v)
		case int64:
			exp = v
		}
		if time.Now().Unix() > exp {
			return nil, errors.New("identity token is expired")
		}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("missing sub claim in identity token")
	}

	if err := verifyExternalAudience(provider, claims); err != nil {
		return nil, err
	}

	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)

	return &ExternalIdentity{
		Sub:   sub,
		Email: email,
		Name:  name,
	}, nil
}

func verifyExternalAudience(provider string, claims jwt.MapClaims) error {
	var envKey string
	switch strings.ToLower(provider) {
	case "google":
		envKey = "BFFX_GOOGLE_CLIENT_ID"
	case "apple":
		envKey = "BFFX_APPLE_CLIENT_ID"
	default:
		return nil
	}
	expected := strings.TrimSpace(os.Getenv(envKey))
	if expected == "" {
		// Dev / mock flows: audience not configured — skip (production should set the env).
		return nil
	}
	if claims["aud"] == nil {
		return errors.New("missing aud claim in identity token")
	}
	for _, part := range strings.Split(expected, ",") {
		want := strings.TrimSpace(part)
		if want != "" && audienceMatches(claims["aud"], want) {
			return nil
		}
	}
	return fmt.Errorf("audience mismatch for provider %s", provider)
}

package authbattery

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ClerkProvider implements AuthBattery using Clerk.com JWKS validation.
// Requires CLERK_JWKS_URL and optionally CLERK_ISSUER.
type ClerkProvider struct {
	jwksURL string
	issuer  string
	kf      keyfunc.Keyfunc
}

// NewClerkProvider creates a new Clerk auth provider.
// It fetches the JWKS from the provided URL and manages periodic background refreshes.
func NewClerkProvider() (*ClerkProvider, error) {
	jwksURL := os.Getenv("CLERK_JWKS_URL")
	if jwksURL == "" {
		return nil, errors.New("CLERK_JWKS_URL is required for clerk auth battery")
	}

	// NewDefault creates a Keyfunc that handles JWKS fetching and background refreshing.
	kf, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("failed to create clerk keyfunc: %w", err)
	}

	return &ClerkProvider{
		jwksURL: jwksURL,
		issuer:  os.Getenv("CLERK_ISSUER"),
		kf:      kf,
	}, nil
}

// ValidateToken verifies the JWT signature using the Clerk JWKS.
func (p *ClerkProvider) ValidateToken(ctx context.Context, tokenString string) (map[string]any, error) {
	if p.kf == nil {
		return nil, errors.New("clerk keyfunc not initialized")
	}
	token, err := jwt.Parse(tokenString, p.kf.Keyfunc)
	if err != nil {
		// Fallback for tests/dev only — disabled in production unless explicitly allowed.
		allowHMACFallback := os.Getenv("BFFX_ENV") != "production" || os.Getenv("BFFX_CLERK_ALLOW_HMAC_FALLBACK") == "true"
		if !allowHMACFallback {
			return nil, fmt.Errorf("clerk token parse failed: %w", err)
		}
		if jwtSecret := os.Getenv("BFFX_JWT_SECRET"); jwtSecret != "" {
			localToken, localErr := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(jwtSecret), nil
			})
			if localErr == nil && localToken.Valid {
				if claims, ok := localToken.Claims.(jwt.MapClaims); ok {
					out := make(map[string]any, len(claims))
					for k, v := range claims {
						out[k] = v
					}
					return out, nil
				}
			}
		}
		return nil, fmt.Errorf("clerk token parse failed: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("clerk token invalid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("clerk claims invalid")
	}

	// Verify issuer if configured.
	if p.issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != p.issuer {
			return nil, fmt.Errorf("clerk issuer mismatch: got %v, want %s", claims["iss"], p.issuer)
		}
	}

	return claims, nil
}

// Issuer returns the configured Clerk issuer.
func (p *ClerkProvider) Issuer() string {
	return p.issuer
}

// UserIDFromClaims extracts the "sub" claim from the Clerk JWT.
func (p *ClerkProvider) UserIDFromClaims(claims map[string]any) string {
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

func (p *ClerkProvider) Type() string {
	return "clerk"
}

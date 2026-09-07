// Package auth provides token-based session validation, policy engine rules (owner, guest, admin),
// and support for built-in JWT and third-party identity providers.
package auth

import "context"

// Provider defines the interface for an authentication battery.
// It allows BFFX to swap between builtin JWT, Clerk, or other OIDC providers
// without changing the mobile contract.
type Provider interface {
	// ValidateToken checks if a token is valid and returns its claims.
	ValidateToken(ctx context.Context, token string) (map[string]any, error)
	
	// Issuer returns the expected issuer for the tokens.
	Issuer() string
	
	// UserIDFromClaims extracts the canonical user ID (usually "sub") from the claims.
	UserIDFromClaims(claims map[string]any) string

	// Type returns the unique identifier for this battery type (e.g. "builtin", "clerk")
	Type() string
}

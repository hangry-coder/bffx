package authbattery

import (
	"github.com/hangry-coder/bffx/pkg/auth"
	"context"
)

// BuiltinProvider implements AuthBattery using the internal JWT service.
type BuiltinProvider struct {
	svc *auth.JWTService
}

func NewBuiltinProvider(svc *auth.JWTService) *BuiltinProvider {
	return &BuiltinProvider{svc: svc}
}

func (p *BuiltinProvider) ValidateToken(ctx context.Context, token string) (map[string]any, error) {
	return p.svc.ValidateToken(ctx, token)
}

func (p *BuiltinProvider) Issuer() string {
	return p.svc.Issuer()
}

func (p *BuiltinProvider) UserIDFromClaims(claims map[string]any) string {
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

func (p *BuiltinProvider) Type() string {
	return "builtin"
}

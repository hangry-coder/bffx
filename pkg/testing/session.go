package testing

import (
	"context"
	"net/http"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
)

// TestSession represents a standardized mock user session in tests
type TestSession struct {
	UserID   string
	Email    string
	Role     string
	Metadata map[string]any
}

// BuildJWT generates an authentic JWT payload signed using the provided secret.
func (s *TestSession) BuildJWT(secret string) (string, error) {
	svc := auth.NewJWTService(secret)
	return svc.GenerateToken(s.UserID, s.Role, "test-device", "test-fingerprint", false, 1*time.Hour)
}

// BuildSupabaseJWT generates an authentic JWT payload structured exactly like Supabase GoTrue claims.
func (s *TestSession) BuildSupabaseJWT(secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":           s.UserID,
		"email":         s.Email,
		"role":          s.Role,
		"user_metadata": s.Metadata,
		"exp":           time.Now().Add(1 * time.Hour).Unix(),
		"iat":           time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// InjectHeader signs a test built-in token and writes it directly to the Request header.
func (s *TestSession) InjectHeader(req *http.Request, secret string) error {
	token, err := s.BuildJWT(secret)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// InjectSupabaseHeader signs a test Supabase token and writes it directly to the Request header.
func (s *TestSession) InjectSupabaseHeader(req *http.Request, secret string) error {
	token, err := s.BuildSupabaseJWT(secret)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// MockClaimsContext creates a context with this session's claims using standard middleware WithClaims.
func (s *TestSession) MockClaimsContext(ctx context.Context) context.Context {
	claims := map[string]any{
		"sub":           s.UserID,
		"email":         s.Email,
		"role":          s.Role,
		"user_metadata": s.Metadata,
	}
	return middleware.WithClaims(ctx, claims)
}

// MockSupabaseContext produces a context structured exactly like Supabase authentications.
func MockSupabaseContext(ctx context.Context, userID, email, role string, metadata map[string]any) context.Context {
	session := &TestSession{
		UserID:   userID,
		Email:    email,
		Role:     role,
		Metadata: metadata,
	}
	return session.MockClaimsContext(ctx)
}

package auth

import (
	"context"
	"testing"
	"time"
)

func TestJWTFlow(t *testing.T) {
	s := NewJWTService("test-secret")
	userID := "user-123"

	token, err := s.GenerateToken(userID, "user", "", "", false, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := s.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	got, _ := claims["sub"].(string)
	if got != userID {
		t.Errorf("expected %s, got %s", userID, got)
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		t.Fatal("expected non-empty jti claim")
	}
}

func TestJWTAnonymousClaim(t *testing.T) {
	s := NewJWTService("test-secret-for-anon-token-flow-xx")
	token, err := s.GenerateToken("guest-1", "guest", "device-a", "", true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := claims["anon"].(bool); !ok || !v {
		t.Fatalf("expected anon=true claim, got %#v", claims["anon"])
	}
}

func TestPasswordHashing(t *testing.T) {
	pass := "secret123"
	hash, err := HashPassword(pass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPasswordHash(pass, hash) {
		t.Errorf("failed to verify password")
	}

	if CheckPasswordHash("wrong", hash) {
		t.Errorf("incorrectly verified wrong password")
	}
}

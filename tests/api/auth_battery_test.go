package api_test

import (
	"context"
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries"
	authbattery "github.com/hangry-coder/bffx/pkg/batteries/auth"
	"github.com/hangry-coder/bffx/pkg/manifest"
	pkgtesting "github.com/hangry-coder/bffx/pkg/testing"

	"gopkg.in/yaml.v3"
)

func TestAuthBatteryBuiltinResolve(t *testing.T) {
	jwtSvc := auth.NewJWTService("test-secret-for-battery")
	spec := &manifest.ProjectSpec{
		Batteries: manifest.BatteriesConfig{Auth: "builtin"},
	}
	got, err := batteries.Resolve(".", spec, nil, jwtSvc, nil, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Auth == nil {
		t.Fatal("expected Auth provider")
	}
	if got.Auth.Type() != "builtin" {
		t.Errorf("Auth.Type() = %q, want builtin", got.Auth.Type())
	}
	bp, ok := got.Auth.(*authbattery.BuiltinProvider)
	if !ok {
		t.Fatalf("Auth type %T, want *BuiltinProvider", got.Auth)
	}
	_ = bp
}

func TestAuthBatteryClerkRequiresJWKS(t *testing.T) {
	spec := &manifest.ProjectSpec{
		Batteries: manifest.BatteriesConfig{Auth: "clerk"},
	}
	jwtSvc := auth.NewJWTService("test-secret")
	_, err := batteries.Resolve(".", spec, nil, jwtSvc, nil, nil)
	if err == nil {
		t.Fatal("expected error when CLERK_JWKS_URL unset")
	}
}

func TestAuthBatteryClerkProviderType(t *testing.T) {
	t.Setenv("CLERK_JWKS_URL", "https://example.com/.well-known/jwks.json")
	p, err := authbattery.NewClerkProvider()
	if err != nil {
		t.Fatalf("NewClerkProvider: %v", err)
	}
	if p.Type() != "clerk" {
		t.Errorf("Type() = %q, want clerk", p.Type())
	}
}

func TestAuthBatteryYAMLDefaultsToBuiltin(t *testing.T) {
	var spec manifest.ProjectSpec
	if err := yaml.Unmarshal([]byte(`batteries: {}`), &spec); err != nil {
		t.Fatal(err)
	}
	jwtSvc := auth.NewJWTService("x")
	got, err := batteries.Resolve(".", &spec, nil, jwtSvc, nil, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Auth.Type() != "builtin" {
		t.Errorf("default auth = %q", got.Auth.Type())
	}
}

func TestTestSessionJWTVerification(t *testing.T) {
	secret := "my-secure-signing-test-key-12345"
	session := &pkgtesting.TestSession{
		UserID: "usr_99",
		Email:  "test@example.com",
		Role:   "editor",
		Metadata: map[string]any{
			"plan": "premium",
		},
	}

	token, err := session.BuildJWT(secret)
	if err != nil {
		t.Fatalf("BuildJWT: %v", err)
	}

	jwtSvc := auth.NewJWTService(secret)
	claims, err := jwtSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims["sub"] != "usr_99" {
		t.Errorf("expected sub usr_99, got %v", claims["sub"])
	}
	if claims["role"] != "editor" {
		t.Errorf("expected role editor, got %v", claims["role"])
	}
}


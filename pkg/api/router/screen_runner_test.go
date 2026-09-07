package router

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func mustYAMLNode(t *testing.T, src string) yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(src), &n); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return *n.Content[0]
	}
	return n
}

func TestRouter_RunScreen_Anonymous(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: yaml.Node{}},
		Screens: []*manifest.Manifest{{
			Kind:     "Screen",
			Metadata: manifest.Metadata{Name: "PublicHome"},
			Spec: mustYAMLNode(t, `
name: PublicHome
nav_type: bottom
order: 1
requires_auth: optional
route: { method: GET, path: /api/v1/screens/public-home }
sources: [app]
output:
  appName: app.name
`),
		}},
		ApiPrefix: "/api/v1",
	}
	store := storage.NewMemoryStore()
	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        store,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(store, reg)),
	}

	res, err := r.RunScreen(context.Background(), "PublicHome", runtimecontracts.RunOpts{})
	if err != nil {
		t.Fatalf("RunScreen returned error: %v", err)
	}
	if res.Errors != nil && len(res.Errors) > 0 {
		t.Fatalf("expected no errors, got %+v", res.Errors)
	}
	got, _ := res.Output["appName"].(string)
	if got != "TestApp" {
		t.Fatalf("expected appName=TestApp, got %q (full: %+v)", got, res.Output)
	}
}

func TestRouter_RunScreen_RequiresAuth_WithoutUserID(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: yaml.Node{}},
		Screens: []*manifest.Manifest{{
			Kind:     "Screen",
			Metadata: manifest.Metadata{Name: "Protected"},
			Spec: mustYAMLNode(t, `
name: Protected
nav_type: bottom
order: 1
requires_auth: required
route: { method: GET, path: /api/v1/screens/protected }
sources: [currentUser]
output:
  user: currentUser
`),
		}},
		ApiPrefix: "/api/v1",
	}
	store := storage.NewMemoryStore()
	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        store,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(store, reg)),
	}

	res, err := r.RunScreen(context.Background(), "Protected", runtimecontracts.RunOpts{})
	if err != nil {
		t.Fatalf("RunScreen returned error: %v", err)
	}
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0], "requires authentication") {
		t.Fatalf("expected auth-required error, got %+v", res.Errors)
	}
}

func TestRouter_RunScreen_Impersonation(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: yaml.Node{}},
		Screens: []*manifest.Manifest{{
			Kind:     "Screen",
			Metadata: manifest.Metadata{Name: "Me"},
			Spec: mustYAMLNode(t, `
name: Me
nav_type: bottom
order: 1
requires_auth: required
route: { method: GET, path: /api/v1/screens/me }
sources: [currentUser]
output:
  user: currentUser
`),
		}},
		ApiPrefix: "/api/v1",
	}
	store := storage.NewMemoryStore()
	created, err := store.Create(context.Background(), "User", map[string]any{"name": "Inspected Alice"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, _ := created["id"].(string)
	if userID == "" {
		t.Fatalf("expected seeded user id; got %+v", created)
	}

	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        store,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(store, reg)),
	}

	res, err := r.RunScreen(context.Background(), "Me", runtimecontracts.RunOpts{UserID: userID})
	if err != nil {
		t.Fatalf("RunScreen error: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("expected no errors, got %+v", res.Errors)
	}
	u, ok := res.Output["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in output, got %+v", res.Output)
	}
	if name, _ := u["name"].(string); name != "Inspected Alice" {
		t.Fatalf("expected name=Inspected Alice, got %q", name)
	}
}

func TestRouter_RunScreen_UnknownScreen(t *testing.T) {
	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: yaml.Node{}},
	}
	r := &Router{reg: reg, mux: http.NewServeMux(), store: storage.NewMemoryStore()}
	if _, err := r.RunScreen(context.Background(), "missing", runtimecontracts.RunOpts{}); err == nil {
		t.Fatal("expected error for unknown screen")
	}
}

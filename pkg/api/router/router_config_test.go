package router

import (
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func testProjectNode(t *testing.T) yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte("app:\n  apiPrefix: /api/v1"), &n); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	return n
}

func TestNormalizeRouterConfig_GroupedDepsOverrideLegacy(t *testing.T) {
	legacyStore := storage.NewMemoryStore()
	groupedStore := storage.NewMemoryStore()
	jwt := auth.NewJWTService("secret")
	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Kind:     "Project",
			Metadata: manifest.Metadata{Name: "cfg-test"},
			Spec:     testProjectNode(t),
		},
	}

	r := NewRouter(RouterConfig{
		Store: legacyStore,
		Core: &RouterCoreDeps{
			Store:        groupedStore,
			Registry:     reg,
			AuthProvider: jwt,
			JWTService:   jwt,
			EventBus:     events.NewMemoryBus(),
		},
	})

	if r.store != groupedStore {
		t.Fatal("expected grouped core store to override legacy Store field")
	}
	if r.reg != reg {
		t.Fatal("expected grouped registry to be applied")
	}
}

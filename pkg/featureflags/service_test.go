package featureflags

import (
	"context"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"

	"gopkg.in/yaml.v3"
)

// stubProvider is a no-op FlagProvider that returns no overrides — every
// section keeps its default_visible value.
type stubProvider struct{}

func (stubProvider) Init(_ ProviderConfig) error                                { return nil }
func (stubProvider) Close() error                                               { return nil }
func (stubProvider) Name() string                                               { return "stub" }
func (stubProvider) Subscribe(_ context.Context, _ func(string, interface{})) error { return nil }
func (stubProvider) Evaluate(_ string, _ EvalContext) (interface{}, error)      { return nil, nil }
func (stubProvider) EvaluateAll(_ EvalContext) (map[string]ResolvedFlag, error) {
	return map[string]ResolvedFlag{}, nil
}
func (stubProvider) GetFlag(_ string) (*FlagDefinition, error) { return nil, nil }
func (stubProvider) ListFlags() ([]FlagDefinition, error)      { return nil, nil }
func (stubProvider) CreateFlag(_ FlagDefinition) (*FlagDefinition, error) {
	return nil, nil
}
func (stubProvider) UpdateFlag(_ string, _ FlagDefinition) (*FlagDefinition, error) {
	return nil, nil
}
func (stubProvider) DeleteFlag(_ string) error { return nil }

func TestResolveScreenSections_DefaultsForLegacyScreens(t *testing.T) {
	var spec yaml.Node
	if err := yaml.Unmarshal([]byte(`name: LegacyScreen
nav_type: bottom
route:
  method: GET
  path: /api/v1/screens/legacy
  auth: required
sources: []
output: {}
`), &spec); err != nil {
		t.Fatalf("legacy screen spec: %v", err)
	}

	reg := &manifest.Registry{
		Screens: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "LegacyScreen"}, Spec: spec},
		},
	}

	svc := &FlagService{
		registry: reg,
		provider: stubProvider{},
		bindings: NewBindingManager(stubProvider{}),
	}
	got := svc.ResolveScreenSections("LegacyScreen", EvalContext{})
	if len(got) != 1 {
		t.Fatalf("expected exactly one synthetic section, got %d (%v)", len(got), got)
	}
	if _, ok := got["default"]; !ok {
		t.Errorf("expected synthetic `default` key, got %v", got)
	}
}

func TestResolveScreenSections_ManifestSectionsHonored(t *testing.T) {
	var spec yaml.Node
	if err := yaml.Unmarshal([]byte(`name: HomeScreen
nav_type: bottom
sources: []
output: {}
sections:
  - key: weekly_stats
    default_visible: true
  - key: hidden_section
    default_visible: false
`), &spec); err != nil {
		t.Fatalf("home screen spec: %v", err)
	}

	reg := &manifest.Registry{
		Screens: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "HomeScreen"}, Spec: spec},
		},
	}

	svc := &FlagService{
		registry: reg,
		provider: stubProvider{},
		bindings: NewBindingManager(stubProvider{}),
	}
	got := svc.ResolveScreenSections("HomeScreen", EvalContext{})
	if len(got) != 2 {
		t.Fatalf("expected 2 sections, got %d (%v)", len(got), got)
	}
	if _, ok := got["weekly_stats"]; !ok {
		t.Errorf("expected weekly_stats section, got %v", got)
	}
	if _, ok := got["default"]; ok {
		t.Errorf("synthetic default must not appear when manifest declares its own sections; got %v", got)
	}
}

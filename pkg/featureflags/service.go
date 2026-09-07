package featureflags

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
)

type FlagService struct {
	registry *manifest.Registry
	provider FlagProvider
	bindings *BindingManager
}

func NewFlagService(reg *manifest.Registry, provider FlagProvider) *FlagService {
	return &FlagService{
		registry: reg,
		provider: provider,
		bindings: NewBindingManager(provider),
	}
}

// ResolveScreenSections returns visibility status for all sections of a screen.
// If the screen manifest does not declare any sections, a single synthetic
// `default` section (visible: true) is returned so callers always get a
// non-empty map and clients can render a fallback container without a
// registry lookup.
func (s *FlagService) ResolveScreenSections(screenName string, ctx EvalContext) map[string]ResolvedSection {
	var sections []manifest.ScreenSectionSpec
	for _, m := range s.registry.Screens {
		var spec manifest.ScreenSpec
		if err := m.UnmarshalSpec(&spec); err == nil && (spec.Name == screenName || m.Metadata.Name == screenName) {
			sections = spec.Sections
			break
		}
	}
	if len(sections) == 0 {
		sections = []manifest.ScreenSectionSpec{{Key: "default", DefaultVisible: true}}
	}
	return s.bindings.ResolveSections(screenName, ctx, sections)
}

// GetValue returns the resolved variation for a flag
func (s *FlagService) GetValue(key string, ctx EvalContext) interface{} {
	val, err := s.provider.Evaluate(key, ctx)
	if err != nil {
		return nil
	}
	return val
}

// GetAllResolved returns all resolved flags for the context
func (s *FlagService) GetAllResolved(ctx EvalContext) map[string]interface{} {
	resolved, err := s.provider.EvaluateAll(ctx)
	if err != nil {
		return make(map[string]interface{})
	}

	results := make(map[string]interface{})
	for k, v := range resolved {
		results[k] = v.Value
	}
	return results
}

// Subscription helper (bridge to provider)
func (s *FlagService) Subscribe(ctx context.Context, onChange func(key string, value interface{})) error {
	return s.provider.Subscribe(ctx, onChange)
}

package providers

import (
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"fmt"
)

// NewProvider returns a configured, initialized FlagProvider based on the
// project specification. It always calls Init on the provider so a missing
// dependency (e.g. LaunchDarkly SDK key) fails loudly at startup rather than
// silently degrading to "all flags evaluate to nil".
func NewProvider(root string, spec *manifest.ProjectSpec, reg *manifest.Registry, store storage.Store) (featureflags.FlagProvider, error) {
	providerType := spec.FeatureFlags.Provider
	if providerType == "" {
		providerType = "bffx"
	}

	cfg := featureflags.ProviderConfig{
		Name:         providerType,
		SDKKey:       spec.FeatureFlags.Config["apiKey"],
		URL:          spec.FeatureFlags.Config["url"],
		Root:         root,
		CustomConfig: make(map[string]interface{}),
	}
	for k, v := range spec.FeatureFlags.Config {
		cfg.CustomConfig[k] = v
	}
	if pk := spec.FeatureFlags.Config["projectKey"]; pk != "" {
		cfg.ProjectKey = pk
	}
	if env := spec.FeatureFlags.Config["environment"]; env != "" {
		cfg.Environment = env
	}

	var p featureflags.FlagProvider
	switch providerType {
	case "launchdarkly":
		p = NewLaunchDarklyProvider(spec.FeatureFlags.Config)
	case "goff":
		p = NewGoffProvider()
	case "bffx":
		p = NewBffxProvider(store, reg)
	default:
		logger.Warn("Unknown features.provider %q; falling back to bffx", providerType)
		p = NewBffxProvider(store, reg)
	}

	if err := p.Init(cfg); err != nil {
		return nil, fmt.Errorf("init flag provider %q: %w", p.Name(), err)
	}
	return p, nil
}

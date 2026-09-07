package ads

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// AdService manages advertisements and mediation identifiers.
type AdService struct {
	reg *manifest.Registry
}

func NewAdService(reg *manifest.Registry) *AdService {
	return &AdService{reg: reg}
}

// ResolveAdUnits returns a mapping of internal ad unit names to store-specific identifiers.
func (s *AdService) ResolveAdUnits(platform string) map[string]string {
	results := make(map[string]string)
	for _, m := range s.reg.AdUnits {
		var spec manifest.AdUnitSpec
		if err := m.UnmarshalSpec(&spec); err == nil {
			id := spec.Identifiers.Ios
			if platform == "android" {
				id = spec.Identifiers.Android
			}
			results[m.Metadata.Name] = id
		}
	}
	return results
}

// GetProviders returns details about all configured ad providers.
type ProviderInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (s *AdService) GetProviders() []ProviderInfo {
	var providers []ProviderInfo
	for _, m := range s.reg.AdProviders {
		var spec manifest.AdProviderSpec
		if err := m.UnmarshalSpec(&spec); err == nil {
			providers = append(providers, ProviderInfo{
				Name: m.Metadata.Name,
				Type: spec.Type,
			})
		}
	}
	return providers
}

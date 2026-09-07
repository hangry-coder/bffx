package featureflags

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// ResolvedSection represents the visibility state of a screen section
type ResolvedSection struct {
	Visible bool   `json:"visible"`
	Flag    string `json:"flag,omitempty"` // The flag key that determined the visibility
}

// BindingManager handles resolving screen section visibility based on flags
type BindingManager struct {
	provider FlagProvider
}

func NewBindingManager(provider FlagProvider) *BindingManager {
	return &BindingManager{provider: provider}
}

// ResolveSections returns a map of section keys to their visibility status for a given screen
func (m *BindingManager) ResolveSections(screenName string, ctx EvalContext, defaultSections []manifest.ScreenSectionSpec) map[string]ResolvedSection {
	results := make(map[string]ResolvedSection)

	// Initialize with defaults
	for _, s := range defaultSections {
		results[s.Key] = ResolvedSection{
			Visible: s.DefaultVisible,
		}
	}

	// Fetch all flags (this could be optimized by querying only flags with bindings for this screen)
	flags, err := m.provider.ListFlags()
	if err != nil {
		return results
	}

	for _, flag := range flags {
		for _, b := range flag.Bindings {
			if b.ScreenName == screenName {
				// Evaluate the flag
				val, err := m.provider.Evaluate(flag.Key, ctx)
				if err != nil {
					continue
				}

				// We assume boolean flags for visibility. 
				// If flag is true, section is visible if scope is "visibility".
				// More complex logic can be added here.
				visible := false
				if bVal, ok := val.(bool); ok {
					visible = bVal
				}

				for _, sectionKey := range b.SectionKeys {
					results[sectionKey] = ResolvedSection{
						Visible: visible,
						Flag:    flag.Key,
					}
				}
			}
		}
	}

	return results
}

package manifest

// NavPreferences mirrors admin/navprefs.Preferences without an import cycle.
type NavPreferences struct {
	Resources map[string]bool
	Features  map[string]bool
}

// ResourcePinned returns whether a resource is pinned (prefs override manifest when set).
func (p NavPreferences) ResourcePinned(resource string, manifestPin bool) bool {
	if p.Resources != nil {
		if v, ok := p.Resources[resource]; ok {
			return v
		}
	}
	return manifestPin
}

// ApplyNavPreferences merges operator pin overrides onto the compiled admin graph.
func ApplyNavPreferences(graph *AdminGraph, prefs NavPreferences) {
	if graph == nil {
		return
	}
	for i := range graph.Resources {
		name := graph.Resources[i].Resource
		if prefs.Resources != nil {
			if v, ok := prefs.Resources[name]; ok {
				graph.Resources[i].Menu.Pin = v
				continue
			}
		}
	}
}

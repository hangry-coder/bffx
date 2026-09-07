package navprefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const fileName = "admin-nav-prefs.json"

// Preferences stores operator sidebar pins (merged with manifest menu.pin on read).
type Preferences struct {
	Resources map[string]bool `json:"resources,omitempty"`
	Features  map[string]bool `json:"features,omitempty"`
}

func pathFor(root string) string {
	return filepath.Join(root, ".bffx", fileName)
}

// Load reads nav preferences from .bffx/admin-nav-prefs.json (empty if missing).
func Load(root string) Preferences {
	p := Preferences{
		Resources: map[string]bool{},
		Features:  map[string]bool{},
	}
	data, err := os.ReadFile(pathFor(root))
	if err != nil {
		return p
	}
	_ = json.Unmarshal(data, &p)
	if p.Resources == nil {
		p.Resources = map[string]bool{}
	}
	if p.Features == nil {
		p.Features = map[string]bool{}
	}
	return p
}

// Save writes preferences to .bffx/admin-nav-prefs.json.
func Save(root string, p Preferences) error {
	if err := os.MkdirAll(filepath.Join(root, ".bffx"), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathFor(root), data, 0o644)
}

// ResourcePinned returns whether a resource is pinned (prefs override manifest when set).
func (p Preferences) ResourcePinned(resource string, manifestPin bool) bool {
	key := strings.TrimSpace(resource)
	if v, ok := p.Resources[key]; ok {
		return v
	}
	return manifestPin
}

// FeaturePinned returns whether a feature nav entry is pinned.
func (p Preferences) FeaturePinned(page string, manifestPin bool) bool {
	key := strings.TrimSpace(page)
	if v, ok := p.Features[key]; ok {
		return v
	}
	return manifestPin
}

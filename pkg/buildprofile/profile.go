// Package buildprofile defines the canonical build profile emitted by bffx sync.
// The profile is the single source of truth for capability detection, doctor
// validation, and minimal-mode vendoring (Phase 7).
package buildprofile

const (
	APIVersion = "bffx.io/v1alpha1"
	Filename   = "build-profile.json"

	ModeFull    = "full"
	ModeMinimal = "minimal"
)

// Profile is written to .bffx/build-profile.json during sync.
type Profile struct {
	APIVersion  string `json:"apiVersion"`
	Project     string `json:"project"`
	Mode        string `json:"mode"`
	GraphHash   string `json:"graphHash,omitempty"`
	ProfileHash string `json:"profileHash,omitempty"`

	Capabilities    Capabilities      `json:"capabilities"`
	Surfaces        RouterSurfaces    `json:"surfaces"`
	Counts          ManifestCounts    `json:"counts"`
	Batteries       BatteriesSnapshot `json:"batteries,omitempty"`
	Packages        []string          `json:"packages"`
	ExcludedTooling []string          `json:"excludedTooling,omitempty"`
}

// Capabilities describe domain surfaces required by manifests and runtime wiring.
type Capabilities struct {
	AI           bool `json:"ai"`
	Game         bool `json:"game"`
	FeatureFlags bool `json:"featureflags"`
	Admin        bool `json:"admin"`
	Billing      bool `json:"billing"`
	Ads          bool `json:"ads"`
	Nutrition    bool `json:"nutrition"`
	Wire         bool `json:"wire"`
	Streaming    bool `json:"streaming"`
	Worker       bool `json:"worker"`
	I18n         bool `json:"i18n"`
}

// RouterSurfaces lists router import surfaces derived from manifests (Phase 7 contract).
type RouterSurfaces struct {
	AddonImports []string `json:"addonImports"`
	Pipelines    []string `json:"pipelines,omitempty"`
	LiveOps      []string `json:"liveops,omitempty"`
	Leaderboards []string `json:"leaderboards,omitempty"`
}

// ManifestCounts summarizes discovered manifest kinds.
type ManifestCounts struct {
	Actions      int `json:"actions"`
	Screens      int `json:"screens"`
	Pipelines    int `json:"pipelines"`
	LiveOps      int `json:"liveops"`
	Leaderboards int `json:"leaderboards"`
}

// BatteriesSnapshot records flag/VLM/nutrition selections relevant to gating.
type BatteriesSnapshot struct {
	Flags     string `json:"flags,omitempty"`
	Vlm       string `json:"vlm,omitempty"`
	Nutrition string `json:"nutrition,omitempty"`
}

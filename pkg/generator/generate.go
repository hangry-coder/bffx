package generator

// This file serves as the main entry point and type registry for the BFFX generator.
// Core logic has been moved to domain-specific files:
// - project.go: Project scaffolding
// - resource.go: Resource & CRUD generation
// - worker.go: Skill & Function logic
// - common.go: Builders, Streams, Actions, and Services

import (
	"path/filepath"

	_ "github.com/hangry-coder/bffx/pkg/generator/mobile" // Ensure mobile sub-package is linked if needed
)

type LayoutType string

const (
	LayoutLegacy LayoutType = "legacy"
	LayoutV2     LayoutType = "v2"
)

type ProjectOptions struct {
	StoreMode        string
	StreamingEnabled bool
	
	AdminEnabled     bool
	AdminEmail       string
	AdminPassword    string

	WithMonetization bool
	WithFlags        bool
	WithTelemetry    bool
	WithAds          bool
	AuthStrategy     string // "mandatory", "optional", "anonymous"
	Minimal          bool
	Layout           LayoutType
	PipelineType     string // Overridden pipeline type (e.g., "chatbot", "ingestion")
	Batteries        *UserBatteryOverrides
	Env              map[string]string
}

// Field represents a data field in a resource.
type Field struct {
	Name string
	Type string
}
type LayoutPaths struct {
	Manifests  string
	Hooks      string
	Blueprints string
	Tests      string
	Actions    string
	Screens    string
	Builders   string
	Templates  string
	CronJobs   string
}

func GetPaths(root, group string, layout LayoutType) LayoutPaths {
	var p LayoutPaths
	if layout == LayoutV2 {
		feature := group
		if feature == "" {
			feature = "app"
		}
		base := filepath.Join(root, "internal", "features", feature)
		p.Manifests = filepath.Join(base, "manifests")
		p.Hooks = filepath.Join(base, "hooks")
		p.Blueprints = filepath.Join(base, "blueprints")
		p.Tests = filepath.Join(base, "tests")
		p.Actions = filepath.Join(base, "actions")
		p.Screens = filepath.Join(base, "screens")
		p.Builders = filepath.Join(base, "builders")
		p.Templates = filepath.Join(base, "templates")
		p.CronJobs = filepath.Join(base, "cronjobs")
		return p
	}

	p.Manifests = filepath.Join(root, "bffx", "resources")
	p.Hooks = filepath.Join(root, "hooks")
	p.Blueprints = filepath.Join(root, "bffx", "blueprints")
	p.Tests = filepath.Join(root, "specs")
	p.Actions = filepath.Join(root, "bffx", "actions")
	p.Screens = filepath.Join(root, "bffx", "screens")
	p.Builders = filepath.Join(root, "bffx", "builders")
	p.Templates = filepath.Join(root, "bffx", "templates")
	p.CronJobs = filepath.Join(root, "bffx", "cronjobs")
	return p
}

package generator

import (
	"fmt"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/generator/archetypes"
)

// ScaffoldArchetype resolves an archetype alias, merges any command line flag overrides,
// scaffolds the project, and runs any post-scaffold pipeline generation hooks.
func ScaffoldArchetype(root, name, alias string, cliOpts ProjectOptions) error {
	arch, ok := archetypes.LookupArchetype(alias)
	if !ok {
		return fmt.Errorf("archetype not found: %s", alias)
	}

	// Create unified options merged from archetype preset + CLI overrides.
	opts := ProjectOptions{
		StoreMode:        arch.Batteries.Store,
		StreamingEnabled: arch.Runtime.Streaming,
		WithMonetization: arch.Addons.Monetization,
		WithFlags:        arch.Addons.Flags,
		WithTelemetry:    arch.Addons.Telemetry,
		WithAds:          arch.Addons.Ads,
		AuthStrategy:     arch.AuthStrategy,
		Minimal:          arch.Minimal,
		Layout:           LayoutType(arch.Layout),
		Batteries: &UserBatteryOverrides{
			Auth:          arch.Batteries.Auth,
			Store:         arch.Batteries.Store,
			Cache:         arch.Batteries.Cache,
			Blob:          arch.Batteries.Blob,
			Analytics:     arch.Batteries.Analytics,
			Observability: arch.Batteries.Observability,
			Flags:         arch.Batteries.Flags,
		},
	}

	// Merging logic: CLI overrides take precedence if they are explicitly set
	if cliOpts.StoreMode != "" {
		opts.StoreMode = cliOpts.StoreMode
		opts.Batteries.Store = cliOpts.StoreMode
	}
	if cliOpts.Minimal {
		opts.Minimal = true
	}
	if cliOpts.AuthStrategy != "" {
		opts.AuthStrategy = cliOpts.AuthStrategy
	}
	if cliOpts.Layout != "" {
		opts.Layout = cliOpts.Layout
	}
	if cliOpts.WithTelemetry {
		opts.WithTelemetry = true
	}
	if cliOpts.WithMonetization {
		opts.WithMonetization = true
	}
	if cliOpts.WithFlags {
		opts.WithFlags = true
	}
	if cliOpts.WithAds {
		opts.WithAds = true
	}
	if cliOpts.AdminEnabled {
		opts.AdminEnabled = true
		opts.AdminEmail = cliOpts.AdminEmail
		opts.AdminPassword = cliOpts.AdminPassword
	}

	if cliOpts.Batteries != nil {
		if cliOpts.Batteries.Auth != "" {
			opts.Batteries.Auth = cliOpts.Batteries.Auth
		}
		if cliOpts.Batteries.Store != "" {
			opts.Batteries.Store = cliOpts.Batteries.Store
			opts.StoreMode = cliOpts.Batteries.Store
		}
		if cliOpts.Batteries.Cache != "" {
			opts.Batteries.Cache = cliOpts.Batteries.Cache
		}
		if cliOpts.Batteries.Blob != "" {
			opts.Batteries.Blob = cliOpts.Batteries.Blob
		}
		if cliOpts.Batteries.Analytics != "" {
			opts.Batteries.Analytics = cliOpts.Batteries.Analytics
		}
		if cliOpts.Batteries.Observability != "" {
			opts.Batteries.Observability = cliOpts.Batteries.Observability
		}
		if cliOpts.Batteries.Flags != "" {
			opts.Batteries.Flags = cliOpts.Batteries.Flags
		}
		if cliOpts.Batteries.Vlm != "" {
			opts.Batteries.Vlm = cliOpts.Batteries.Vlm
		}
	}

	opts.Env = cliOpts.Env

	// 1. Run core scaffolding
	if err := ScaffoldNewProject(root, name, opts); err != nil {
		return fmt.Errorf("scaffold base project: %w", err)
	}

	// 2. Post-Scaffold Pipeline Hook
	projectDir := filepath.Join(root, name)
	if arch.Pipeline != nil || cliOpts.PipelineType != "" {
		pType := ""
		pName := ""
		pFeature := ""
		if arch.Pipeline != nil {
			pType = arch.Pipeline.Type
			pName = arch.Pipeline.Name
			pFeature = arch.Pipeline.Feature
		}
		if cliOpts.PipelineType != "" {
			pType = cliOpts.PipelineType
		}
		if pName == "" {
			pName = "Default"
		}
		if pFeature == "" {
			pFeature = "system"
		}

		err := GeneratePipeline(projectDir, pName, pFeature, pType, "", opts.Layout)
		if err != nil {
			return fmt.Errorf("generate post-scaffold pipeline: %w", err)
		}
	}

	return nil
}


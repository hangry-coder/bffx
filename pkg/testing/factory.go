package testing

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"fmt"
)

type Trait map[string]any

type Factory struct {
	store      storage.Store
	blueprints map[string]*manifest.BlueprintSpec
	traits     map[string]map[string]Trait // resourceName -> traitName -> fields
}

func NewFactory(store storage.Store, reg *manifest.Registry) *Factory {
	f := &Factory{
		store:      store,
		blueprints: make(map[string]*manifest.BlueprintSpec),
		traits:     make(map[string]map[string]Trait),
	}
	if reg != nil {
		for _, m := range reg.Blueprints {
			var spec manifest.BlueprintSpec
			m.UnmarshalSpec(&spec)
			f.blueprints[m.Metadata.Name] = &spec
		}
	}

	f.registerBuiltinTraits()
	return f
}

func (f *Factory) registerBuiltinTraits() {
	// Users
	f.RegisterTrait("User", "admin", Trait{"role": "admin", "status": "active"})
	f.RegisterTrait("User", "guest", Trait{"role": "guest", "status": "active"})
	f.RegisterTrait("User", "banned", Trait{"role": "user", "status": "banned"})

	// AdminUsers
	f.RegisterTrait("AdminUser", "superadmin", Trait{"permissions": "*"})
	f.RegisterTrait("AdminUser", "content_editor", Trait{"permissions": "app,mobile"})
	f.RegisterTrait("AdminUser", "system_only", Trait{"permissions": "system"})

	// FeatureFlags
	f.RegisterTrait("FeatureFlag", "enabled", Trait{"enabled": true, "rollout": 100})
	f.RegisterTrait("FeatureFlag", "disabled", Trait{"enabled": false, "rollout": 0})
	f.RegisterTrait("FeatureFlag", "partial_rollout", Trait{"enabled": true, "rollout": 50})
}

// RegisterTrait adds a named variation for a specific resource
func (f *Factory) RegisterTrait(resource, name string, fields Trait) {
	if f.traits[resource] == nil {
		f.traits[resource] = make(map[string]Trait)
	}
	f.traits[resource][name] = fields
}

// TraitBuilder allows chaining traits before creation
type TraitBuilder struct {
	factory  *Factory
	resource string
	active   []string
}

func (f *Factory) WithTrait(traitName string) *TraitBuilder {
	return &TraitBuilder{factory: f, active: []string{traitName}}
}

func (tb *TraitBuilder) WithTrait(traitName string) *TraitBuilder {
	tb.active = append(tb.active, traitName)
	return tb
}

func (tb *TraitBuilder) Build(overrides map[string]any) map[string]any {
	return tb.factory.Build(tb.resource, tb.mergeTraits(overrides))
}

func (tb *TraitBuilder) Create(overrides map[string]any) (map[string]any, error) {
	return tb.factory.Create(tb.resource, tb.mergeTraits(overrides))
}

// Helper to set resource context for builder (usually called internally by factory helper methods if they existed)
func (tb *TraitBuilder) For(resource string) *TraitBuilder {
	tb.resource = resource
	return tb
}

func (tb *TraitBuilder) mergeTraits(overrides map[string]any) map[string]any {
	result := make(map[string]any)
	for _, tName := range tb.active {
		if tFields, ok := tb.factory.traits[tb.resource][tName]; ok {
			for k, v := range tFields {
				result[k] = v
			}
		}
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

// Build returns a map representing the resource with default values
func (f *Factory) Build(name string, overrides map[string]any) map[string]any {
	spec, ok := f.blueprints[name]
	
	result := make(map[string]any)
	
	// 1. Start with blueprint defaults if they exist
	if ok {
		for k, v := range spec.Fields {
			result[k] = v
		}
	}

	// 2. Apply smart faker defaults for missing fields if we have a blueprint
	if ok {
		// Identify fields in the actual resource that aren't in the result yet
		// For now we just use the name as a hint to generate a payload
		// In a real scenario we'd look at the Resource manifest fields
	}

	// 3. Apply overrides
	for k, v := range overrides {
		result[k] = v
	}

	// 4. Fill in common gaps with faker if still empty and likely required
	// This is a simplified version of the SMART logic
	if result["email"] == nil && (name == "User" || name == "AdminUser") {
		result["email"] = FakeValue("email")
	}
	if result["name"] == nil && (name == "User" || name == "AdminUser") {
		result["name"] = FakeValue("name")
	}

	return result
}

// Create builds and saves the resource to the test store
func (f *Factory) Create(name string, overrides map[string]any) (map[string]any, error) {
	data := f.Build(name, overrides)
	
	// Use the resource name from blueprint if available, otherwise assume name is resource name
	resourceName := name
	if spec, ok := f.blueprints[name]; ok {
		resourceName = spec.Resource
	}

	record, err := f.store.Create(context.Background(), resourceName, data)
	if err != nil || record == nil {
		return nil, fmt.Errorf("failed to create record for %s: %w", resourceName, err)
	}

	return record, nil
}

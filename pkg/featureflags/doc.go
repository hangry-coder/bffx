// Package featureflags provides the core feature-flag evaluation contract used by
// the router, admin, and screen bindings.
//
// Provider adapters (bffx manifest table, goff, LaunchDarkly) live in
// pkg/featureflags/providers and are selected via batteries.flags in project.yaml.
// See docs/core-concepts/extension_taxonomy.md for the core vs battery split.
package featureflags

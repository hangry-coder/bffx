// Package manifest handles the loading, validation, and typing of the BFFX YAML graph.
//
// Core Concepts:
//   - Registry: The central container for all loaded manifests (resources, screens, actions, etc).
//   - LoadAll: Recursively walks the bffx/ directory and unmarshals YAML into generic Manifest structs.
//   - UnmarshalSpec: Decodes Manifest.Spec (yaml.Node) into typed structs like ResourceSpec.
//
// Extension Points:
//   - Adding a new Kind (e.g. "Feed"): Update manifest/registry.go to track it and manifest/spec.go for its typed spec.
//   - Validation: Add logic to reg.Validate() to catch cross-resource reference errors.
//   - Linting: Add security or style rules to manifest/linter.go.
package manifest

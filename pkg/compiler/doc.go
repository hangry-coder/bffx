// Package compiler bridges manifests to generated artifacts: Sync walks the project,
// hashes the registry, reconciles schema, and emits the orchestrator/OpenAPI surface.
// Entry points are Sync and related helpers; codegen is invoked from here after the
// manifest graph is validated.
package compiler

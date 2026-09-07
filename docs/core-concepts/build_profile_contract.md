---
title: Build Profile Contract
description: Canonical build profile emitted by sync for packaging, doctor validation, and runtime gating.
category: core-concepts
---

# Build Profile Contract (Phase 7)

`bffx sync` emits **`.bffx/build-profile.json`** as the single source of truth for:

- capability detection (`ai`, `game`, `featureflags`, `admin`, …),
- router addon import surfaces (`pkg/featureflags`, `pkg/addons/billing`, …),
- minimal-mode selective `.bffx/core` vendoring,
- doctor drift/consistency checks,
- runtime startup warnings when the profile is stale.

## Schema

| Field | Meaning |
|-------|---------|
| `mode` | `full` (default) or `minimal` — set via `spec.packaging.mode` in `bffx/project.yaml` |
| `graphHash` | Matches `.bffx/graph.hash` from the same sync |
| `profileHash` | SHA-256 of the profile (excluding itself) for drift detection |
| `capabilities` | Derived booleans from manifests + batteries |
| `surfaces.addonImports` | Router import surfaces for AI/game/addons |
| `packages` | `pkg/` paths vendored in minimal mode (nil/empty in full mode = copy all) |
| `excludedTooling` | CLI packages skipped in minimal vendoring |

## Derivation Rules

- **`capabilities.ai`**: any `Pipeline` manifest **or** `batteries.vlm` ≠ `noop`
- **`capabilities.game`**: any `LiveOpsEvent` or `Leaderboard` manifest
- **`capabilities.featureflags`**: flags provider configured or idempotency/refresh-token features enabled
- **`capabilities.admin`**: `spec.admin.enabled`
- **`capabilities.nutrition`**: `batteries.nutrition` ≠ `noop`

## Minimal Vendoring

When `mode=minimal`, `bffx update framework --vendor-only` copies `packages` instead of the entire `pkg/` tree. Tooling paths (`pkg/compiler`, `pkg/generator`, `pkg/doctor`, …) are excluded because the runtime orchestrator does not import them.

Full mode remains the compatibility-safe default.

## Related docs

- [Architecture contracts](architecture_contracts.md) — program index
- [Extension taxonomy](extension_taxonomy.md) — batteries vs addons
- [CLI: migrate packaging](../reference/cli.md#bffx-migrate-packaging-planapplyrollback---root-dir---to-fullminimal---json)
- [Project manifest: spec.packaging](project_manifest.md#-specpackaging)

## Validation

`bffx doctor` checks:

1. profile exists (warn if missing — run `bffx sync`),
2. `profileHash` / `graphHash` match a fresh derive from manifests,
3. internal consistency (e.g. pipelines present ⇒ `capabilities.ai`).

## Shadow Profile Validation

Shadow profile validation derives and validates reference project profiles (admin, wire, streaming, featureflags, AI enabled; game off until LiveOps/Leaderboard manifests are added).

## Migration (Phase 8)

Switch packaging mode with guided migration:

```bash
bffx migrate packaging plan          # preflight + dry-run diff
bffx migrate packaging apply --to minimal
bffx migrate packaging rollback      # restore last backup
```

Rollback metadata lives in `.bffx/packaging-migration.json`; backups under `.bffx/backups/packaging-migration/`.

CI runs `scripts/ci/packaging-matrix.sh` on every PR to validate full vs minimal scaffolds, build-profile emission, and sync/build thresholds.

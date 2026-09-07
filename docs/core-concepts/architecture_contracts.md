---
title: Architecture Contracts (Modernization Baseline)
description: Dependency direction, package taxonomy, and local-first provider adapter contracts for cache and observability.
category: core-concepts
---

# Architecture Contracts (Phase 0 Baseline)

This document defines the architecture contracts used by the modernization plan. It is the source of truth for refactoring boundaries and compatibility gates.

## 1. Dependency Direction Contract

Required direction:

1. `manifest` and `storage` are foundational and must not depend on `api/router` or `admin`.
2. `api/router` can depend on neutral contracts, but not admin-owned DTOs/types.
3. `admin` may depend on neutral runtime contracts, never the reverse.
4. `app/server` is composition-only and should wire subsystems through grouped dependencies and adapters.

Target: remove boundary inversion while preserving behavior through staged migration.

### Package ownership map (Wave 6 baseline)

| Package | Owns | Must not depend on |
|---------|------|---------------------|
| `pkg/manifest` | YAML kinds, registry, admin graph compile | `pkg/api/router`, `pkg/admin` |
| `pkg/storage` | Store drivers, schema reconcile, migrations | `pkg/admin` |
| `pkg/auth` | JWT, policy engine, credential helpers | `pkg/admin` |
| `pkg/runtimecontracts` | Neutral screen/kill-switch contracts | `pkg/admin`, `pkg/api/router` |
| `pkg/api/middleware` | HTTP cross-cutting (auth, rate limit, CSP) | `pkg/admin` |
| `pkg/api/router` | Public REST/screens/actions/pipelines | `pkg/admin` (use `runtimecontracts`) |
| `pkg/admin` | Admin SPA API, resource handlers, liveops | — (may import router contracts via adapters) |
| `pkg/app` | Server composition, wiring batteries | Prefer interfaces over admin DTOs |
| `pkg/batteries/*` | Provider adapters (cache, blob, flags, wire) | Domain addons |
| `pkg/addons/*` | Product capabilities (billing, nutrition, game) | Should not become second router |

Enforced in CI: `go test ./pkg/api/router/ -run TestProductionCode_DoesNotImportAdmin` and public `/api/docs` stays off middleware bypass lists (see `pkg/api/middleware/middleware_test.go`).

## 2. Taxonomy Contract

Three-way taxonomy:

- `core`: runtime and contract primitives (manifest, router contracts, storage/auth interfaces, observability semantics).
- `batteries`: infrastructure provider adapters and swappable connectors (cache backends, logging backends, flag providers).
- `addons`: domain capability packages (billing, ads, catalog, game services, AI feature surfaces).

`featureflags` classification in this program:
- Core evaluation semantics and contract live in `pkg/featureflags`.
- Provider-specific adapters live in `pkg/featureflags/providers` and are selected via `batteries.flags`.

Detailed Phase 6 taxonomy and decision checklist: [`extension_taxonomy.md`](extension_taxonomy.md).

## 3. Cache Contract (Local-First + Provider Adapters)

Two-layer model:

- **Core semantics layer**: keying, TTL behavior, invalidation rules, safety/fail policy semantics.
- **Provider adapter layer**: memory/redis/upstash implementations.

Invariant: application-visible cache behavior must remain equivalent across local and third-party providers.

Detailed Phase 3 contract and usage map: [`cache_contract.md`](cache_contract.md).

## 4. Observability Contract (Local-First + Provider Adapters)

Two-layer model:

- **Core telemetry semantics**: canonical event names, metric keys/labels, trace correlation behavior.
- **Provider adapter layer**: slog/axiom/sentry/posthog emitters and transports.

Invariant: naming and semantic meaning must not drift by provider.

Detailed Phase 4 contract and usage map: [`observability_contract.md`](observability_contract.md).

## 5. Build Profile Contract (Phase 7)

`bffx sync` emits `.bffx/build-profile.json` as the canonical capability and vendoring profile.

- **Full mode** (default): vendoring copies all of `pkg/`.
- **Minimal mode** (`spec.packaging.mode: minimal`): vendoring copies runtime `packages` and excludes CLI tooling paths.

Doctor validates profile drift and manifest consistency on every run.

Detailed Phase 7 contract: [`build_profile_contract.md`](build_profile_contract.md).

## 6. Pilot Compatibility Gates

Generated backends consumed by mobile clients must preserve strict wire contracts. During refactors:

- API contract baselines are frozen prior to breaking changes.
- Compatibility smoke gates must remain green phase-by-phase.
- Shadow profile validation must pass before cutover.
- Rollback path stays available for at least one release cycle after cutover.

## 7. Baseline Measurement Harness

Baseline scripts for modernization program:

| Script | When it runs | Purpose |
|--------|----------------|---------|
| `scripts/ci/architecture-baseline.sh` | Every PR (+ schedule/manual) | Sync/build/binary-size + `.bffx/core` bytes for reference fixtures |
| `scripts/ci/packaging-matrix.sh` | Every PR | Full vs minimal scaffold, migration tests, sync/build thresholds |

## 8. Packaging Migration & Release Gates (Phase 8)

Guided opt-in to minimal packaging (default remains **full** for compatibility):

```bash
bffx migrate packaging plan
bffx migrate packaging apply --to minimal
bffx sync && bffx update framework --vendor-only && bffx doctor
# Rollback if needed:
bffx migrate packaging rollback
```

Doctor enforces build-profile integrity (drift, consistency, required profile when `packaging.mode=minimal`) and surfaces rollback pointers in `.bffx/packaging-migration.json`.

See [Build Profile Contract](build_profile_contract.md) and [CLI: migrate packaging](../reference/cli.md#bffx-migrate-packaging-planapplyrollback---root-dir---to-fullminimal---json).

---
title: Extension Taxonomy (Phase 6)
description: Batteries vs addons vs core — where new capabilities belong.
category: core-concepts
---

# Extension Taxonomy (Phase 6)

BFFX uses a three-way taxonomy for runtime extensions. Use this document when adding or moving code so generated projects stay modular and doctor checks stay meaningful.

## 1) Core (`pkg/*` contracts + runtime semantics)

**Purpose:** framework behavior, neutral contracts, evaluation semantics.

Examples:

- `pkg/manifest`, `pkg/storage`, `pkg/api/router` orchestration
- `pkg/runtimecontracts` — admin/public shared DTOs
- `pkg/cache`, `pkg/observability` — canonical semantics + fail policies
- `pkg/featureflags` — flag evaluation contract, bindings, `EvalContext`, resolved flag shape

**Rule:** core packages define *what* the framework means, not *which vendor* backs it.

## 2) Batteries (`pkg/batteries/*`)

**Purpose:** swappable **infrastructure** providers selected in `bffx/project.yaml` under `spec.batteries`.

| Battery key | Package | Examples |
|-------------|---------|----------|
| `auth` | `pkg/batteries/auth` | builtin, clerk |
| `cache` | `pkg/batteries/cache` | memory, redis, upstash |
| `blob` | `pkg/storage/blob` | local, s3, r2 |
| `observability` | `pkg/batteries/observability` | slog, axiom, sentry |
| `analytics` | `pkg/batteries/analytics` | noop, posthog |
| `vlm` | `pkg/batteries/vlm` | noop, gemini, ollama |
| `flags` | `pkg/featureflags/providers` | bffx, goff, launchdarkly |

**Rule:** if the choice is "which backend/vendor for a cross-cutting infra concern", it is a battery.

Feature flags split (finalized in Phase 6):

- **Core:** `pkg/featureflags` (`FlagProvider` interface, evaluation, screen bindings)
- **Battery adapters:** `pkg/featureflags/providers/*` (bffx manifest table, goff, LaunchDarkly)

## 3) Addons (`pkg/addons/*`)

**Purpose:** optional **domain/product** capabilities composed by the router or hooks.

Examples:

- `pkg/addons/billing` — Stripe entitlements
- `pkg/addons/ads` — ad unit resolution
- `pkg/addons/catalog/nutrition` — Open Food Facts catalog for ingestion pipelines
- `pkg/game/*` — leaderboards, liveops (game domain)

**Rule:** if the code implements product behavior or a domain API surface (not infra wiring), it is an addon.

## 4) Migration: deprecated battery → addon

| Legacy battery | Replacement | Notes |
|----------------|-------------|-------|
| `batteries.nutrition` | Ingestion pipeline + `spec.catalog.adapter: openfoodfacts` | Use `pkg/addons/catalog/nutrition` via `resolvePipelineCatalog` in router |

Doctor warns when `batteries.nutrition` is non-`noop` (see `lintPipelines` / `lintExtensionTaxonomy`).

## 5) Decision checklist

1. **Vendor swap for infra?** → battery + register in `pkg/batteries/registry.go`
2. **Domain feature used by routes/hooks?** → addon under `pkg/addons/<domain>/`
3. **Shared contract/DTO used across admin + public?** → `pkg/runtimecontracts` or relevant core contract package
4. **New flag provider?** → implement `featureflags.FlagProvider` in `pkg/featureflags/providers/`, wire in `flagproviders.NewProvider`

## 6) Related docs

- [`architecture_contracts.md`](architecture_contracts.md) — dependency direction + taxonomy summary
- [`build_profile_contract.md`](build_profile_contract.md) — capabilities and minimal vendoring surfaces
- [`../batteries/index.md`](../batteries/index.md) — manifest battery keys
- [`../extending/writing_batteries.md`](../extending/writing_batteries.md) — implement a new battery adapter

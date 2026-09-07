# Pluggable batteries

BFFX keeps the BFF thin: manifests, router, policies, and hooks stay in your repo. **Batteries** are swappable **infrastructure** providers for auth, storage, cache, blobs, flags, observability, analytics, and optional AI backends — selected in `bffx/project.yaml` and configured via `config/<env>.yaml` plus environment variables.

**Not batteries:** domain/product capabilities (billing, ads, game services, catalog adapters) live under **`pkg/addons/`**. See the [extension taxonomy](../core-concepts/extension_taxonomy.md) for the full batteries vs addons vs core model.

**Precedence:** `bffx/project.yaml` (which battery) → `config/<env>.yaml` (endpoints, secrets) → `BFFX_*` env vars.

## Schema (`spec.batteries`)

```yaml
batteries:
  auth: builtin          # builtin | clerk
  store: sqlite          # sqlite | postgres (alias: spec.store.mode)
  cache: memory          # memory | redis | upstash
  blob: local            # local | minio | r2 | s3
  analytics: noop        # noop | posthog
  observability: slog    # slog | axiom | axiom_sentry
  flags: bffx            # bffx | goff | launchdarkly
  vlm: noop              # noop | ollama | gemini | openrouter
  nutrition: noop        # [DEPRECATED] use Ingestion Pipeline catalog adapter (see extension_taxonomy.md)
  i18n: builtin          # builtin | none
```

**Feature flags:** evaluation contract is core (`pkg/featureflags`); provider adapters (`bffx`, `goff`, `launchdarkly`) are wired via `batteries.flags` — see [extension taxonomy](../core-concepts/extension_taxonomy.md#2-batteries-pkgbatteries).

Run **`bffx doctor`** to see resolved batteries and missing env vars.

Battery selections are reflected in **`.bffx/build-profile.json`** (emitted by `bffx sync`) under `capabilities` and `batteries`. See [Build Profile Contract](../core-concepts/build_profile_contract.md).

## Defaults (offline-first)

| Battery | Default | Local dev |
|---------|---------|-----------|
| auth | `builtin` | JWT + refresh in-process |
| store | `sqlite` | File under `.bffx/` |
| cache | `memory` | In-process (or `redis` when `runtime.redis.enabled`) |
| blob | `local` | `.bffx/uploads/` |
| analytics | `noop` | No outbound calls |
| observability | `slog` | Stdout |
| flags | `bffx` | Manifest + optional admin table, RAM evaluation |
| vlm | `noop` | Clear error unless configured |
| nutrition | `noop` | [DEPRECATED] Use pipeline catalog addon — see [extension taxonomy](../core-concepts/extension_taxonomy.md) |

Cloud batteries (Clerk, Upstash, R2, PostHog, Axiom, Gemini, LaunchDarkly) are **opt-in** — see per-battery docs below.

## Per-battery docs

| Doc | Topic |
|-----|--------|
| [flags.md](flags.md) | Internal `bffx`, goff, LaunchDarkly |
| [postgres.md](postgres.md) | Postgres / Supabase URL as store |

Vendor bridges (not batteries): [supabase.md](../supabase.md), [posthog.md](../posthog.md), [firebase.md](../firebase.md).

## Built-in Modules & Addons

BFFX includes pre-built modules and engines that you can toggle on/off:

### Admin Panel (`spec.admin.enabled: true`)

The BFFX Admin Panel is a first-class, manifest-driven administration console served at `/admin`.
* **Zero Node Runtime**: Relies on a pre-compiled React SPA embedded directly into the Go binary. App developers never need to manage Node or npm dependencies.
* **Declarative Configuration**: Layouts, columns, search filters, scopes, and dashboards are defined completely in YAML manifests under `bffx/admin/`.
* **Go Action Hooks**: Connects custom UI buttons and batch actions directly to type-safe Go Hooks.
* **Security Gating**: The interactive API Reference (Swagger UI) is moved inside the authenticated admin panel in non-production, and permanently disabled in production mode.

## Verdict

**Beta** — Used in `examples/notes` and production pilots. Swap providers by YAML + env only; no hook rewrites required for auth/store/cache/blob/flags.

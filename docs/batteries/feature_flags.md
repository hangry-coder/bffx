# Feature flags battery

Mobile apps must **not** evaluate rollouts. The BFF resolves flags server-side and exposes a flat map on **`GET /api/v1/app/bootstrap`** under `features.flags`.

## Providers

| `batteries.flags` | Use when | Config |
|-------------------|----------|--------|
| **`bffx`** (default) | Flags in `bffx/**/FeatureFlag` YAML + optional admin overrides | Store battery for `bffx_feature_flag` table |
| **`goff`** | GitOps file or HTTP-hosted `flags.yaml` | `featureFlags.config.url` or `flagsFile: ./config/flags.yaml`, `pollingInterval: 10s` |
| **`launchdarkly`** | Team already on LD | `LAUNCHDARKLY_SDK_KEY` or `featureFlags.config.apiKey` |

**PostHog is not a flag provider** — use the [analytics](../posthog.md) battery only.

## `bffx` provider (default)

- Loads manifest flags + DB overrides into **RAM** on init (`Reload()`); hot path does not query SQL per request.
- Targeting, % rollouts, prerequisites: `pkg/featureflags/evaluator.go`.

## `goff` provider

- Polls `config/flags.yaml` or remote URL; admin CRUD for flags is **file/Git**, not `/admin/flags`.
- CI: use checked-in `testdata/flags.yaml`, no network.

## `launchdarkly` provider

- SDK initializes **asynchronously** after startup (does not block `ListenAndServe` for 5s).
- Missing SDK key when LD is selected → **fail at `Init`** / doctor **fail**.

## Verdict

**Beta** — `bffx` + `goff` + LD wired in `pkg/featureflags/providers/`. OpenFeature is used inside goff/LD adapters; mobile keeps bootstrap-only contract.

# BFFX Compatibility & Versioning Policy

This document outlines versioning guarantees, manifest schema evolution, package API tiers, and conditional compiling via build tags.

---

## 1. Semantic Versioning in v0.x (Pre-1.0)

BFFX follows [Semantic Versioning 2.0.0](https://semver.org/). However, during the initial public beta phase (version `v0.x.y`):
- **Minor releases (`v0.X.0`)** may introduce breaking API surface changes, new CLI workflows, or structural manifest changes.
- **Patch releases (`v0.x.Y`)** are reserved for bug fixes, performance optimizations, security patches, and backward-compatible additions.

We highly recommend pinning your `bffx` CLI version and framework vendor version to the exact release tag during the beta phase.

---

## 2. Manifest API Versioning (`v1alpha1`)

The main project and resource manifests currently target:
```yaml
apiVersion: bffx.io/v1alpha1
```

### Schema Evolution
- The schema will transition to `v1beta1` and ultimately `v1` as core configurations stabilize.
- Breaking manifest changes will include automated migrations (e.g., `bffx migrate layout` or custom migration helpers) to adapt existing projects.

---

## 3. Package API Tiers

BFFX packages in `pkg/` are categorized into three stability tiers to set clear expectations for external developers and code contributors:

| Tier | Packages | Stability / Coverage | Breaking Policy |
| :--- | :--- | :--- | :--- |
| **Stable / Production** | `pkg/storage`, `pkg/auth`, `pkg/api/router`, `pkg/manifest`, `pkg/api/handlers` | Production-ready. High unit/integration test coverage (>50%). | Interfaces remain stable. Breaking changes require deprecation notices in patch/minor cycles. |
| **Beta** | `pkg/featureflags`, `pkg/batteries`, `pkg/comm`, `pkg/deploy` | Functional, used in active pilot projects. Structural testing present. | Minor adjustments allowed during minor releases. Interface modifications are documented in the CHANGELOG. |
| **Experimental** | MongoDB and PocketBase `Store` drivers (`pkg/storage/mongo.go`, etc.) | Proof-of-concept. Limited tests. Subject to panic or incomplete behavior. | Subject to change or removal at any time without warning. |

---

## 4. Conditional Compilation (Build Tags)

BFFX keeps runtime dependencies lean by gating non-essential features and experimental drivers behind Go build tags:

| Tag | Purpose | Affected Package |
| :--- | :--- | :--- |
| `sentry` | Compiles Sentry SDK hooks into the observability pipeline. | `pkg/observability` |
| `experimental_mongo` | Enables the MongoDB `Store` driver. | `pkg/storage` |
| `experimental_pocketbase` | Enables the PocketBase integration `Store` driver. | `pkg/storage` |

### Compilation Example
To build or run tests with Sentry enabled:
```bash
go build -tags=sentry ./cmd/orchestrator
```

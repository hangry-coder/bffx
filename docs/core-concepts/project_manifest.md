# ⚙️ BFFX Project Configuration Reference (`project.yaml`)

The `project.yaml` file is the master manifest for your BFFX application. It defines your technology stack, security policies, and architectural defaults.

---

## 🔝 Top-Level Fields

| Field | Description | Example |
| :--- | :--- | :--- |
| `apiVersion` | The version of the BFFX manifest schema. | `bffx.io/v1alpha1` |
| `kind` | Must be `Project` for this manifest. | `Project` |
| `metadata.name` | The internal name of your project. | `my-app` |

---

## 🛠 `spec.runtime`
Defines the execution environment for your API, background workers, and real-time features.

### `api`
*   **`language`**: The language of the orchestrator (currently `go`).
*   **`port`**: The port the server listens on (default: `8080`).

### `worker`
*   **`enabled`**: Set to `true` to enable Python-based background jobs and AI skills.
*   **`language`**: The implementation language (currently `python`).

### `redis`
*   **`enabled`**: Required if using professional-grade background job queues or real-time event buses.
*   **`url`**: The connection string (e.g., `redis://localhost:6379`).

### `streaming`
*   **`enabled`**: Enables Server-Sent Events (SSE) for real-time dashboard updates and notifications.

---

## 💾 `spec.store` & `spec.telemetryStore`
Configures where your application data and telemetry logs are persisted. You can split these into different databases (e.g., SQLite for app data, MongoDB for telemetry).

| Mode | Description | Required Fields |
| :--- | :--- | :--- |
| `memory` | Transient in-memory store. Lost on restart. | None |
| `sqlite` | Embedded database. Best for single-node apps. | `path` |
| `pocketbase` | Hybrid local/cloud backend. | `url`, `apiKey` |
| `postgres` | Production-grade relational store. | `url` |
| `mongo` | High-volume document store. | `url` |

*   **`path`**: Path to the database file (for `sqlite`). Default: `.bffx/app.db`.
*   **`url`**: Connection string for remote databases.

---

## 📦 `spec.defaults`
Global switches for built-in framework "batteries."

*   **`auth`**: Set to `builtin` to enable the internal User/Session system.
*   **`providers`**: List of OAuth2 providers (e.g., `[google, apple]`).
*   **`files`**: Enables automatic file upload/bucket management.
*   **`jobs`**: Enables the `/api/v1/jobs` endpoints for tracking background tasks.
*   **`builders`**: Enables declarative Data Builders (Aggregated API views).

---

## 🛡 `spec.security`
Production hardening settings.

### `rateLimit`
*   **`requestsPerMinute`**: How many requests a single IP can make per minute.
*   **`burstSize`**: Maximum sudden spikes allowed before dropping requests.

### `appSecret`
*   A shared secret that mobile clients must send in the `X-App-Secret` header to handshake with the API. This prevents unauthorized direct API access.

### `allowedIPs`
*   A list of IP addresses or CIDR ranges allowed to access the API. If empty, all IPs are allowed.

### `allowedOrigins`
*   List of domains allowed to make CORS requests (e.g., `["https://myapp.com"]`). Use `["*"]` for open development.

### `admin`
*   **`allowedIPs`**: A restricted list of IPs allowed to access the Admin Web UI. Highly recommended for production.

---

## 🤖 `spec.jobs`
*   **`retentionDays`**: How many days to keep background job logs in the database before pruning (default: `7`).

---

## 📱 `spec.app`
*   **`namespace`**: The bundle identifier/package name (e.g., `com.example.myapp`).
*   **`apiPrefix`**: The versioning prefix for all generated routes (default: `/api/v1`).

---

## 💎 Addons & Extensions

*   **`admin.enabled`**: Enables the Built-in Admin Web UI at `/admin`.
*   **`monetization.enabled`**: Enables the `Entitlement` system and billing verification endpoints.

---

## 📦 `spec.packaging`

Controls build profile mode and selective framework vendoring (see [Build Profile Contract](build_profile_contract.md)).

| Field | Values | Default | Description |
| :--- | :--- | :--- | :--- |
| `mode` | `full`, `minimal` | `full` | **Full** vendors all of `pkg/` into `.bffx/core`. **Minimal** vendors runtime packages only and excludes CLI tooling paths. |

`bffx sync` always emits `.bffx/build-profile.json` reflecting the effective mode and derived capabilities. Use `bffx migrate packaging` to switch modes on existing projects with backup/rollback.

---

## 🔋 `spec.batteries`

Infrastructure provider selections (not domain addons — see [Extension Taxonomy](extension_taxonomy.md)).

| Key | Examples | Role |
| :--- | :--- | :--- |
| `auth` | `builtin`, `clerk` | Identity provider |
| `store` | `sqlite`, `postgres` | Primary store (aliases `spec.store.mode`) |
| `cache` | `memory`, `redis`, `upstash` | HTTP/tag cache backend |
| `blob` | `local`, `s3`, `r2` | File storage |
| `analytics` | `noop`, `posthog` | Product analytics |
| `observability` | `slog`, `axiom`, `sentry`, `axiom_sentry` | Logs/traces (see [observability contract](observability_contract.md)) |
| `flags` | `bffx`, `goff`, `launchdarkly` | Feature flag provider |
| `vlm` | `noop`, `gemini`, `ollama`, `openrouter` | Vision/LLM battery for AI pipelines |
| `nutrition` | `noop`, `openfoodfacts` | **Deprecated** — prefer ingestion pipeline catalog adapters |
| `i18n` | `builtin`, `none` | Localization |

Full battery reference: [Batteries Overview](../batteries/index.md).

---

## 🚦 `spec.featureFlags`
Configures the Feature Flag and Remote Config engine (core semantics in `pkg/featureflags`; providers via `batteries.flags`).

*   **`provider`**: Alias of `batteries.flags`. Options: `bffx` (default), `goff`, `launchdarkly`.
*   **`idempotency`**: Enables `X-Idempotency-Key` support for Actions/POSTs.
*   **`refreshTokens`**: Opt out of built-in refresh-token resource injection when set to `false`.
*   **`config`**: Provider-specific settings (e.g. LaunchDarkly `apiKey`).

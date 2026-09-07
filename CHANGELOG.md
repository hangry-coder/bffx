# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Phase 3 Authoritative Leaderboards (ZSET primary, SQL fallback, secure nonces, anti-spam, resets)
- **`bffx new` CLI:** `--store` flag (`sqlite`, `postgres`, `mongo`, `memory`, `pocketbase`) with store-aware `project.yaml` and `.env`; `--parent-dir` as clearer alias for parent output path.
- **CLI Help Flags:** Native support for `help`, `--help`, and `-h` commands with standard exit code 0.

### Changed
- **`bffx new` defaults:** **v2** layout is now the default (`internal/features/`, `cmd/api/`). Use `--layout legacy` for the older tree. Projects are created as `./NAME/` in the cwd without requiring `--root .` or `--layout v2`.
- **Dependency Modernization:** Updated core framework dependencies (`golang.org/x/crypto v0.56.0`, `golang.org/x/net v0.58.0`, `golang.org/x/sync v0.22.0`, `github.com/redis/go-redis/v9 v9.22.0`, `modernc.org/sqlite v1.58.0`, `go.opentelemetry.io/otel v1.46.0`).
- **CI Modernization:** Cleaned up CI workflows to remove external pilot dependencies and focus strictly on open source framework tests.

### Security Hardening
- **User Scaffold Restriction**: Restructured default `User` resource scaffolding policy to `write: authenticated`, preventing unauthorized registration/escalation via raw CRUD.
- **Resource Write Hardening**: Updated default policies for `AppString` and `AppConfig` scaffolds to `write: admin` to restrict configuration modification to administrators.
- **CRUD Response Sanitization**: Added a response filter that automatically strips sensitive fields (`password`, `password_hash`, `secret_token`, `otp_code`) from CRUD response payloads.
- **MCP HTTP Fail-Closed Authentication**: Enabled strict fail-closed token validation when serving Model Context Protocol (MCP) over HTTP.
- **MCP Path Traversal Guard**: Added path sanitization and jail check validation to restrict MCP tool directory access.
- **Environment Secrets Validation**: Added strict fail-fast validation for minimum key lengths and default values of core secrets (`BFFX_JWT_SECRET`, `BFFX_WORKER_SECRET`, `BFFX_APP_SECRET`, `BFFX_ADMIN_SESSION_KEY`).
- **Gated Swagger API Docs**: Removed the public `/api/docs` and `/api/docs/openapi.json` endpoints. Embedded Swagger UI is now hosted under the authenticated admin SPA (`/api/admin/docs` and `/api/admin/docs/openapi.json`) and permanently disabled in production.

## [0.1.3-beta] - 2026-05-26

### Added
- **Production Posture Doctor Gates**: Added production checks (`bffx doctor --env production`) to identify SQLite-in-production, local filesystem blob storage in production, unresolved `${VAR}` variables, and plaintext credentials in blueprints/seeds.
- **Dev-Only Routing Protection**: Supported `dev_only: true` flag in manifests to skip endpoint registration when `BFFX_ENV=production`.
- **Forced HTTPS & HSTS Middleware**: Added `app.forceSSL` toggle to redirect plain HTTP requests to HTTPS and emit HSTS headers when terminated upstream.
- **Hardened CI/CD Scaffolding**: Default CI/CD template now runs static analysis (vet + `golangci-lint`), vulnerability scanner (`govulncheck`), preflight doctor configuration checks, and post-deployment smoke health checks.
- **Reverse Proxy Deployment Scaffolding**: Added default Caddy reverse proxy container sidecar to `docker-compose.prod.yml` and automatic `Caddyfile` generation to terminate TLS/SSL.

### Changed
- **Secure Scaffolding Defaults**: Updated resource scaffolding default policy from `owner` (which can allow unauthenticated access if not restricted) to `authenticated` for read and write.

### Fixed
- **Secrets Exposure Fix**: Removed the rule copying `.env` files into generated docker images inside the Dockerfile generator.

## [0.1.2-beta] - 2026-05-19

### Added
- **Modernized Admin UI (V2)**: Fully redesigned, high-fidelity React Single Page Application served directly via `go:embed`. Features dynamic resource registration and group discovery.
- **Admin Authentication Gate**: Integrated robust cookie-based session login overlay (`bffx_admin_session`) and auto-redirection on HTTP 401 unauthorized calls.
- **System-Table Prefixing**: Standardized core database tables (`user`, `device`, `adminuser`, etc.) to use the `bffx_` namespace (`bffx_user`, `bffx_device`, `bffx_admin_user`, etc.) to separate framework tables from application domain models.
- **Dynamic Table Resolver**: Implemented custom middleware table-name mapping in storage drivers supporting dual-read backward-compatible queries.
- **Form Field Stripping**: Excluded backend auto-generated fields (`id`, `created_at`, `updated_at`, etc.) from interactive edit modals.
- **CLI Commands**: Added `bffx sync` command to compile and propagate schema changes, and `bffx migrate` commands to manage database state.

## [0.1.1-beta] - 2026-05-18

### Added
- **AI Pipelines (Beta)**: Native orchestrator for Multi-modal Ingestion (`type: ingestion`) and conversational Chatbots (`type: chatbot`).
- **Chatbot Memory Manager**: Sliding window conversation rolling-session memory backed by HSL-optimized tagCache.
- **Server-Sent Events (SSE)**: Standard SSE chunk token streaming for real-time model responses.
- **Asynchronous Execution**: Support for `execution: async` enqueuing tasks in background worker queues with StatusAccepted.
- **CLI Scaffolding**: Support for Rails-like `bffx generate pipeline <type>` to auto-scaffold pipeline manifests, adapters, hooks, and prompts.

### Deprecated
- **Core Batteries**: Deprecated `batteries.nutrition` key in project configuration in favor of local `Ingestion Pipelines` with custom `Catalog` adapters.

## [0.1.0-beta] - 2026-05-11

### Added
- **Core Engine**: Manifest-driven Go backend generator.
- **Routing**: Automatic RESTful CRUD routing for resources.
- **Screens & Builders**: Dynamic UI section resolution for mobile clients.
- **Authentication**: JWT-based auth with refresh token rotation and anonymous sessions.
- **Storage**: Multi-driver support including SQLite, MemoryStore, and Postgres (Beta).
- **Hardening**: 10-week sprint completion with >55% coverage on core packages.
- **Integrations**: FCM (Push), SMTP (Email), S3 (File storage).
- **Developer Experience**: CLI tool (`bffx sync`, `bffx dev`) and documentation.
- **Example**: Comprehensive `notes` example application.

### Changed
- Refactored monolithic router into modular `pkg/api/router`.
- Migrated feature flag system to provider-based architecture.

### Fixed
- Resolved nil-pointer panics in auth middleware.
- Fixed migration runner state management for concurrent deployments.

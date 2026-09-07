# BFFX Security Inventory

This document tracks sensitive configuration, public attack surface, and security controls for production BFFX deployments. Keep it aligned with runtime behavior when shipping security changes.

## Environment secrets

**NEVER** commit actual values for these keys to version control.

| Variable | Description | Default (Dev) | Required |
| --- | --- | --- | --- |
| `BFFX_JWT_SECRET` | Signs and verifies JWT access tokens (min 32 chars). | Scaffolded dev default | Yes |
| `BFFX_WORKER_SECRET` | Authenticates worker write-backs (min 32 chars). | Scaffolded dev default | Yes (if jobs/worker enabled) |
| `BFFX_APP_SECRET` | `X-App-Secret` middleware (min 16 chars). | Scaffolded dev default | Yes (production) |
| `BFFX_ADMIN_SESSION_KEY` | Signs admin session cookies (min 32 chars). | Scaffolded dev default | Yes |
| `BFFX_MCP_TOKEN` | Required for MCP over HTTP (`bffx mcp serve --transport http`). | Unset (stdio OK) | Yes (HTTP MCP) |
| `BFFX_DB_PASSWORD` | Postgres password when not using `DATABASE_URL`. | N/A | Yes (managed Postgres) |
| `BFFX_REDIS_PASSWORD` | Redis auth when cache/events use Redis. | N/A | Recommended (prod) |
| `BFFX_GOOGLE_CLIENT_ID` | Expected `aud` for Google ID tokens (OAuth link flows). | Unset (skipped) | Recommended (prod OAuth) |
| `BFFX_APPLE_CLIENT_ID` | Expected `aud` for Apple ID tokens. | Unset (skipped) | Recommended (prod OAuth) |
| `CLERK_JWKS_URL` | Clerk JWKS endpoint when `batteries.auth: clerk`. | N/A | Yes (Clerk mode) |

## Startup safeguards

- **Fail-fast (production):** Missing or short secrets abort startup when `BFFX_ENV=production`.
- **Weak secret warnings:** Default scaffold secrets log warnings in non-production.
- **Doctor:** `bffx doctor --env production` flags SQLite-in-prod, local blob storage, and insecure resource policies (e.g. `User` with `write: public`).

## Public API surface

| Surface | Production behavior |
| --- | --- |
| **`/api/docs`** | **Not registered** on the public router. OpenAPI/Swagger is served under the admin API when `features.api_docs` is enabled (dev only; hard-off in production). |
| **CRUD responses** | Sensitive fields stripped via router sanitization (`password`, `password_hash`, `otp_code`, `secret_token`, …). |
| **MCP HTTP** | Fail-closed without valid `BFFX_MCP_TOKEN`; constant-time comparison. |
| **Auth login/signup** | Per-IP and per-email lockout (`BFFX_AUTH_LOGIN_MAX_ATTEMPTS`, `BFFX_AUTH_LOGIN_LOCKOUT_MINUTES`). |
| **Refresh tokens** | Stored as bcrypt hashes; wire format `rt1.{id}.{secret}` (legacy plaintext rows still accepted until rotated). |
| **Clerk battery** | HS256 fallback to `BFFX_JWT_SECRET` **disabled** in production unless `BFFX_CLERK_ALLOW_HMAC_FALLBACK=true`. |
| **Anonymous auth** | Optional RPM limit (`BFFX_ANONYMOUS_AUTH_RPM`); disable with `BFFX_DISABLE_ANONYMOUS_ENDPOINT=true`. |

## Scaffold defaults (new projects)

| Resource | Policy | Notes |
| --- | --- | --- |
| `User` | `read: owner`, `write: authenticated` | Signup via `/auth/*`, not public CRUD POST |
| `AppString` / `AppConfig` | `write: admin` | CMS/config not world-writable |
| `Device` | `write: authenticated` (or owner) | Registration requires auth |

## Admin

- Session cookies: signed payload with optional absolute TTL and idle timeout (see admin session config).
- Non-admin roles cannot obtain admin sessions (login rejects viewer/demo accounts).
- Admin routes are separate from public CRUD; rate limits bypass health/metrics/admin static paths where configured.

## Deployment

- Do not bake `.env` into images; inject secrets at runtime.
- Mount SQLite data at `/app/.bffx/data/` only (not over `/app/.bffx/` — masks vendored core).
- Use immutable image tags (`BFFX_DEPLOY_TAG`) for rollback; see `docs/operations/runbook.md`.

## Related docs

- [`docs/operations/runbook.md`](../operations/runbook.md) — deploy, rollback, operations
- [`docs/getting-started/state_of_the_project.md`](../getting-started/state_of_the_project.md) — stability matrix and limitations
- [`docs/core-concepts/architecture_contracts.md`](../core-concepts/architecture_contracts.md) — extension taxonomy

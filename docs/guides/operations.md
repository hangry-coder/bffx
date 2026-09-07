# BFFX Operations Guide

This guide covers the necessary steps to deploy and maintain a BFFX application in a production environment.

## 1. TLS Termination

BFFX server runs as a standard HTTP server. In production, you **MUST** terminate TLS before traffic reaches the BFFX process. We recommend using a reverse proxy like **Caddy** or an **AWS ALB**.

### Caddy Configuration
```caddy
api.yourdomain.com {
    reverse_proxy localhost:8080 {
        header_up Host {host}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }
}
```

### Trusted Proxy Headers
Ensure you set `BFFX_TRUST_X_FORWARDED_FOR=true` in your environment so BFFX correctly attributes client IPs for rate limiting and audit logs.

## 2. Structured Logging

For production observability, enable JSON logging:

```bash
BFFX_LOG_FORMAT=json
```

This will change access logs to a structured format:
```json
{"timestamp":"2026-05-10T12:00:00Z","level":"INFO","request_id":"...","method":"GET","path":"/api/v1/posts","status":200,"duration":"12ms","ip":"1.2.3.4"}
```

In text mode, the same fields are emitted in a stable order
(`request_id`, `method`, `path`, `status`, `duration`, `ip`, then any extras
alphabetically) so `grep`/`cut`/`awk` cuts stay predictable across processes.

## 3. SQLite Online Backups

If you are using the SQLite storage driver, you should perform regular online backups using the built-in `VACUUM INTO` command.

### Manual Backup
Run the following CLI command:
```bash
bffx db backup --path ./backups/app_$(date +%F).db
```

### Automated Backups (Cron)
Add a cron job to your server to run backups daily:
```bash
0 2 * * * /usr/local/bin/bffx db backup --path /mnt/backups/bffx/app_$(date +\%F).db >> /var/log/bffx-backup.log 2>&1
```

## 4. Production Environment Variables

Ensure these are set in your production environment:

| Variable | Recommended Value | Description |
|---|---|---|
| `BFFX_ENV` | `production` | Enables security hardening (fail-closed CORS, etc.) |
| `BFFX_JWT_SECRET` | `(high-entropy string)` | Mandatory for token signing |
| `BFFX_ADMIN_SESSION_KEY` | `(high-entropy string)` | Mandatory for admin dashboard security |
| `BFFX_CORS_ALLOW_ORIGINS` | `https://app.yourdomain.com` | Restricted origin list |
| `BFFX_REDIS_URL` | `redis://...` | Required for token revocation and distributed rate limiting |
| `BFFX_SCHEMA_SINGLETON` | unset or `true` | When not `false`, legacy `Reconcile()` uses a Postgres advisory lock or a SQLite mutex so two processes do not race DDL. |

Add provider-specific keys per your manifest. Common BFFX env vars:

| Variable | Notes |
|---|---|
| `BFFX_FCM_SERVICE_ACCOUNT_JSON` / `BFFX_FCM_SERVICE_ACCOUNT_PATH` | Firebase service account JSON for FCM HTTP v1 (`BFFX_FCM_PROJECT_ID` optional override). |
| `BFFX_SMTP_HOST`, `BFFX_SMTP_PORT`, `BFFX_SMTP_USER`, `BFFX_SMTP_PASS`, `BFFX_SMTP_FROM` | Outbound SMTP (takes precedence over Resend when `BFFX_SMTP_HOST` is set). |
| `RESEND_API_KEY`, `BFFX_RESEND_FROM` | Resend HTTP API (10s client timeout; non-2xx surfaces typed errors). |
| `BFFX_S3_ENDPOINT`, `BFFX_S3_ACCESS_KEY`, `BFFX_S3_SECRET_KEY`, `BFFX_S3_BUCKET` | S3-compatible presigned PUT/GET (`BFFX_S3_REGION`, `BFFX_S3_USE_SSL` optional). |
| `BFFX_ENABLE_LOCAL_UPLOADS`, `BFFX_LOCAL_UPLOAD_SECRET` | In **production**, local HMAC presign + on-disk store requires explicit enable; non-production defaults to `.bffx/uploads` under the project root. |
| `BFFX_POSTHOG_API_KEY`, `BFFX_POSTHOG_HOST` | Enables batched server-side analytics (PostHog `/batch/`). Host optional (default `https://app.posthog.com`). |
| `BFFX_STRIPE_WEBHOOK_SECRET` | Registers `POST /api/v1/billing/stripe/webhook` with Stripe signature verification (`whsec_...`). |
| `BFFX_ENABLE_OAUTH` | Set to `true` to register mock Google OAuth routes (`/api/v1/auth/google*`) for development only. |
| `BFFX_SENTRY_DSN`, `BFFX_RELEASE` | Used only when built with `-tags sentry` (see `pkg/observability`). If Sentry initialization fails (e.g., bad/invalid DSN), BFFX logs a warning and fails soft, bypassing Sentry without crashing. |

## 5. Schema startup locking

For SQL stores using the **declarative reconcile** fallback (no `migrations/` directory yet), the orchestrator calls `Store.Reconcile` on boot. With `BFFX_SCHEMA_SINGLETON=false`, locking is skipped (useful for tests). The default behavior reduces concurrent DDL races; rolling deploys should still prefer the versioned `bffx migrate` path for Postgres.

## 6. PR integration stack (`docker-compose.test.yml`)

The repo root [`docker-compose.test.yml`](../docker-compose.test.yml) starts **Postgres** (port **5433** on the host), **Redis**, and **MinIO** for local parity with CI. Pull-request CI runs:

```bash
docker compose -f docker-compose.test.yml up -d
export DATABASE_URL=postgres://bffx:bffx@127.0.0.1:5433/bffx?sslmode=disable
go test -tags=integration -count=1 ./tests/integration/...
```

Golden output for the notes example registry can be refreshed with:

```bash
go test ./tests/golden -run TestGoldenNotesRegistry -update-notes-golden
```

## 7. Operational Database Migrations & Rollback Procedures

### 7.1 Running Migrations in Production
Before upgrading your application server container, run the migration engine:
```bash
bffx migrate apply
```
If this command fails:
1. It returns exit code `1`.
2. It locks the `schema_migrations` state as `dirty`.
3. The server process will fail to start until the migration is manually fixed and marked clean.

### 7.2 System-Table Prefixing Compatibility Mode
In v0.1.2, system tables are prefixed with `bffx_`. The framework supports safe backward-compatible rollbacks:
1. **Dual-Read Safe:** If a newer version writes to a new table but you roll back to an older version of the framework, set:
   ```env
   BFFX_SYSTEM_TABLES_COMPAT_READ=true
   ```
   This ensures the older codebase falls back to reading from legacy tables if the prefixed table contains partial data.
2. **Old Table Retention:** Do **NOT** drop the old non-prefixed tables (like `user`, `device`) until at least two successful minor release cycles have passed in production.

## 8. API Reference & Swagger Gating

For security and attack-surface reduction, the interactive API reference (Swagger UI) is restricted as follows:

* **No Public Access**: The public `/api/docs` and `/api/docs/openapi.json` routes are completely removed.
* **Admin-Authenticated Only**: In development and staging, the API Reference is accessible only from the Admin Panel (`/admin/`) under the "API Reference" page. This requires a valid admin session cookie.
* **Permanently Disabled in Production**: When `BFFX_ENV=production`, all API Reference endpoints under `/api/admin/docs` are completely disabled and return a `404 Not Found` response, even for logged-in superadministrators. This behavior cannot be overridden.

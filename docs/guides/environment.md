# 🔑 BFFX Environment Configuration

BFFX uses environment variables for both the core framework CLI and the runtime services.

## Core Framework Secrets
These are required for the Admin Panel and security features.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `BFFX_ADMIN_SECRET` | Master secret key for the Admin Dashboard. | (Required) |
| `BFFX_JWT_SECRET` | Secret key for signing JWT tokens. | (Auto-generated if missing) |

## Infrastructure & Addons
Optional configuration for connecting to external services.

| Variable | Description |
| :--- | :--- |
| `BFFX_REDIS_URL` | Connection string for Redis EventBus. (e.g., `redis://localhost:6379`) |
| `BFFX_POSTGRES_URL` | Connection string for PostgreSQL Store. |
| `BFFX_POCKETBASE_URL` | URL for PocketBase storage adapter. |
| `BFFX_TELEGRAM_TOKEN` | Bot token for the Telegram communication channel. |

## Security & JWT (runtime API)

| Variable | Description |
| :--- | :--- |
| `BFFX_APP_SECRET` | If set, clients must send matching `X-App-Secret` on `/api/v1/*` and `/auth/*`. Required in production per env validation. |
| `BFFX_WORKER_SECRET` | Bearer token for worker write-back endpoints. |
| `BFFX_JWT_ISS` | JWT issuer (`iss`) claim on new tokens; validated when present or when `BFFX_JWT_REQUIRE_ISS_AUD=true`. |
| `BFFX_JWT_AUD` | JWT audience (`aud`) claim on new tokens; same validation rules as issuer. |
| `BFFX_JWT_REQUIRE_ISS_AUD` | When `true`, tokens must include valid `iss` and `aud`. |
| `BFFX_BCRYPT_COST` | Password hashing cost (default `12`, clamped 10–15). |
| `BFFX_TRUST_X_FORWARDED_FOR` | When `true`, rate limiting uses `X-Forwarded-For` / `X-Real-IP` (only behind a trusted LB). |
| `BFFX_DISABLE_ANONYMOUS_AUTO_PROVISION` | When `true`, middleware does not create guest users from `X-Device-ID` alone. |
| `BFFX_DISABLE_ANONYMOUS_ENDPOINT` | When `true`, `POST .../auth/anonymous` returns 503. |
| `BFFX_ANONYMOUS_AUTH_RPM` | Per-IP requests/minute cap for anonymous session creation (default `20`). |

JWTs include a `jti` claim for revocation hooks (`JWTService.SetRevocationChecker`).

## Development Mode
When running `bffx dev`, you can export these locally:

```bash
export BFFX_ADMIN_SECRET=your_secret_here
bffx dev
```

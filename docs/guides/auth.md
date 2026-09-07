# Authentication & sessions

## Refresh tokens

When the project manifest does **not** define a `RefreshToken` resource, `bffx` injects a built-in resource at load time so `POST /api/v1/auth/refresh` (and the legacy `/auth/refresh` path) can persist rotated refresh tokens with `family_id`, `used`, and related fields.

### Opting out

In the **Project** manifest under `spec.features`, set:

```yaml
features:
  refreshTokens: false
```

Then no built-in `RefreshToken` resource is added. In that configuration, refresh-token rotation endpoints return **501** unless you declare your own `RefreshToken` (or equivalent) resource.

### Default

If `refreshTokens` is omitted, it defaults to **enabled** so new scaffolds work with refresh rotation without extra YAML.

## Logout and revocation hardening

- `POST /api/v1/auth/logout` now revokes the current JWT `jti` and also revokes refresh-token session state.
- If you include `{"refresh_token":"<token>"}` in logout, BFFX revokes the whole refresh-token family for that session.
- If no `refresh_token` is provided, BFFX revokes refresh tokens for the current `(user, device)` session when available.

### Production defaults

- In production, revocation should be **fail-closed** (`BFFX_JWT_REVOCATION_FAIL_CLOSED=true` or unset).
- Configure Redis for durable denylisting (`runtime.redis.enabled: true` and `BFFX_REDIS_ADDR`).
- `bffx doctor` now flags unsafe production postures (missing Redis revocation backend or fail-open override).

## Google OAuth (development mock)

BFFX does **not** ship production Google ID-token verification in-tree. The historical `/api/v1/auth/google` and `/api/v1/auth/google/callback` handlers are **mock** JSON responses used for wiring tests and pilots.

- Routes are registered **only** when `BFFX_ENABLE_OAUTH=true`.
- With the variable unset (default), those paths are **not registered** and return **404** from the mux.

For a real Google sign-in flow, integrate your IdP or extend the auth handler with JWKS verification and keep this flag off until that code is merged.

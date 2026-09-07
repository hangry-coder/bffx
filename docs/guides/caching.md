# BFFX Smart Cache: Caveats & Uncovered Paths

> **Contract reference:** canonical cache keying, TTL, and invalidation semantics are defined in [Cache Contract](../core-concepts/cache_contract.md). This guide covers operational caveats.

The BFFX Smart Cache provides declarative, tag-based invalidation. However, it is not a "perfect" cache consistency engine. Developers must be aware of paths that do NOT trigger automatic invalidation.

## Uncovered Paths (Auto-Invalidation Bypass)

The following operations **do not** trigger `OnWrite` callbacks and will leave the cache stale until the TTL expires:

1. **Direct SQL Mutations**: Any write performed via `db.Exec`, `db.Query`, or external tools (psql, DBeaver) bypasses the `storage.Store` layer.
2. **Worker Python Mutations**: If you use Python sidecars or external workers that write directly to the database, they must manually call the BFFX Invalidation API or wait for TTL.
3. **Internal Helper Writes**: If a developer uses the raw `*sql.DB` or `ent.Client` inside a hook without going through `r.store`, invalidation will not fire.
4. **Batch Operations**: Large batch updates performed outside the standard `Create/Update/Delete` store methods.
5. **Soft Deletes**: If soft-deletion logic is implemented purely via SQL `UPDATE` without notifying the store of the semantic "delete".

## Best Practices

- **Standardize on `storage.Store`**: Always use the store for business-level mutations.
- **Coarse Tags**: Use resource-level tags (e.g., `resource:Note`) for lists, and specific tags (e.g., `resource:Note:123`) for detail screens.
- **Manual Invalidation**: For uncovered paths, inject the `tagcache.Tagger` and call `InvalidateTags(ctx, []string{"resource:MyResource"})` explicitly.
- **Short TTLs**: When in doubt, use shorter TTLs (e.g., 30-60s) to limit the impact of stale cache.

## Environment: `BFFX_TAG_CACHE_INVALIDATE`

When Redis tag cache is enabled, successful Create/Update/Delete through `storage.InvalidatingStore` normally calls `InvalidateTags` for `resource:<Name>` and `resource:<Name>:<id>`.

- **`BFFX_TAG_CACHE_INVALIDATE=false`**: Skips that **`InvalidateTags`** path only (the `OnWrite` callback returns early). Use when the invalidation bridge causes load or correctness issues while you investigate.
- **What still runs**: CRUD and actions that opt into `route.invalidate_cache` still call `middleware.InvalidatePattern` with **`bffx:action-cache:*`**, which clears only the **opt-in action GET** cache (`middleware.Cache`), not tag-backed screen entries (`bffx:cache:k:…` / `bffx:cache:t:…` in `pkg/cache/tagcache`).

There is no built-in “nuke all HTTP caches” env flag. For an operational emergency, operators can run targeted **`InvalidatePattern`** (or Redis admin) against **`bffx:cache:k:*`** and **`bffx:cache:t:*`** after understanding blast radius.

## Response header

Cacheable surfaces set **`X-BFFX-Cache`** to **`HIT`**, **`MISS`**, or **`BYPASS`** (see `pkg/api/middleware/cache.go` and `pkg/api/router/helpers_cache.go`).

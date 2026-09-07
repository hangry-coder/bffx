---
title: Cache Contract (Phase 3)
description: Canonical cache semantics and provider adapter rules.
category: core-concepts
---

# Cache Contract (Phase 3)

This document defines the canonical cache contract for BFFX runtime code and provider adapters.

## 1) Two-Layer Model

- **Core semantics layer**
  - Namespaces, keying, TTL behavior, and invalidation semantics.
  - Owned by `pkg/cache` and runtime usage in `pkg/api/*`.
- **Provider adapter layer**
  - Backend-specific implementations for memory/redis/upstash.
  - Owned by `pkg/batteries/cache/*` (and legacy `pkg/cache/tagcache` compatibility).

## 2) Canonical Namespaces

Defined in `pkg/cache/provider.go`:

- `RedisKeyPrefix` (`bffx:cache:k:`) — provider-level data key namespace.
- `RedisTagPrefix` (`bffx:cache:t:`) — provider-level tag-set namespace.
- `ActionResponseKeyPrefix` (`bffx:action-cache:`) — logical action GET cache keys.
- `TaggedResponseKeyPrefix` (`bffx:tagcache:`) — logical screen/builder tagged cache keys.
- `TagIndexPrefix` (`cache_index:`) — secondary index namespace used by indexed provider.

## 3) Canonical Key/Tag Builders

Core helpers:

- `cache.BuildHTTPScopedKey(...)` for deterministic HTTP response keys.
- `cache.NormalizeOwner(...)` for owner scoping (`anon` fallback).
- `cache.RedisDataKey(...)` and `cache.RedisTagKey(...)` for provider-level key assembly.
- `cache.TagIndexKey(...)` for indexed-provider tag index keys.
- `cache.ScreenTag(...)` and `cache.SectionTag(...)` for screen/section invalidation tags.

## 4) Invalidation Semantics

- **Pattern invalidation**
  - Used for action cache (`middleware.InvalidateActionCache` with `ActionCacheInvalidatePattern`).
  - Must only target action logical keys under `ActionResponseKeyPrefix`.
- **Tag invalidation**
  - Used for screen/builder/section cache via tagged writes.
  - Screen and section invalidation paths use canonical tags (`screen:<name>`, `screen:<name>:section:<key>`).

## 5) Fail Policy Matrix

- **Get miss or backend unavailable**
  - Runtime treats as miss/fail-open where supported.
- **Set/Invalidate backend failure**
  - Runtime continues request flow; logs warn/error based on path.
  - Provider adapters should avoid crashing request paths.

## 6) Phase 3 Usage Map (Primary Paths)

- Core contracts: `pkg/cache/provider.go`, `pkg/cache/httpkey.go`, `pkg/cache/index.go`.
- Router tagged cache wiring: `pkg/api/router/helpers_cache.go`.
- Action cache middleware: `pkg/api/middleware/cache.go`, `pkg/api/middleware/action_cache.go`.
- Provider adapters: `pkg/batteries/cache/redis.go` (and peers), legacy compatibility `pkg/cache/tagcache/*`.

## 7) Related docs

- [Architecture contracts](architecture_contracts.md) — program index
- [Caching guide (operational caveats)](../guides/caching.md)
- [Extension taxonomy](extension_taxonomy.md)

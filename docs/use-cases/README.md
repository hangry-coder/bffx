---
title: BFFX use cases
description: End-to-end recipes for common product shapes built on BFFX manifests, CLI, and batteries.
category: use-cases
---

# BFFX use cases

Recipes for building **complete product shapes** with BFFX — not framework tutorials, but **command sequences**, manifest patterns, and verification checklists you can follow from zero to production.

Each guide assumes **v2 layout** (`internal/features/`, `cmd/api/`, `db/migrations/`) unless noted.

| Guide | What you build | Frontend |
|-------|----------------|----------|
| [Headless CMS (2026 WordPress alternative)](headless-cms-wordpress-alternative.md) | Content API + operator admin; markdown posts/pages | React / Next.js / any TS client |

**Related docs**

- [CLI reference](../reference/cli.md)
- [Admin manifest](../core-concepts/admin_manifest.md)
- [Resource manifest](../core-concepts/resource_manifest.md)
- [REST-only clients](../mobile/rest_client.md)

**Contributing a new use case**

1. Add `docs/use-cases/<slug>.md` with: goal, architecture diagram, CLI steps, sample manifests, verify checklist.
2. Link it from this index.
3. Prefer commands that exist in `bffx --help` today; mark aspirational steps clearly.

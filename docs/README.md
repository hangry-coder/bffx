---
title: Documentation Map
description: Central visual index and navigation hub for all BFFX architectural guides, tutorials, and operations manuals.
---

# 📖 BFFX Documentation Map

Welcome to the BFFX knowledge base. This master index organizes all framework documentation into clear, intent-driven categories to help you build, test, deploy, and scale your backends.

---

## 🟢 1. Getting Started
*For new users onboarding to the framework:*
- **[Quickstart Guide](getting-started/quickstart.md)** — Bootstrap a new application, generate resources, and launch the dev server in three commands.
- **[Vertical Archetypes](getting-started/quickstart.md#⚡️-quick-start-with-vertical-archetypes)** — Pre-configured industry presets for fintech, commerce, dictation, and superapp to accelerate development.
- **[State of the Project](getting-started/state_of_the_project.md)** — Stability matrix of subsystems, capabilities showroom, and the Supabase/BaaS vs BFFX decision tree.
- **[Migration Guide](getting-started/migration.md)** — Database schema migrations (`bffx migrate init/plan/apply`) and **packaging migration** (`bffx migrate packaging`) for full ↔ minimal profiles.

---

## 🧠 2. Core Concepts
*Understand how the BFFX orchestrator and compiler think:*
- **[System Architecture](core-concepts/architecture.md)** — Design philosophy, resource-action-hook lifecycle flows, and horizontal scaling tiers.
- **[Architecture Contracts (Modernization)](core-concepts/architecture_contracts.md)** — Dependency direction, compatibility gates, CI baselines, and links to all contract specs.
- **[Build Profile Contract](core-concepts/build_profile_contract.md)** — `.bffx/build-profile.json`, full vs minimal packaging, migration, and vendoring.
- **[Extension Taxonomy](core-concepts/extension_taxonomy.md)** — Core vs batteries vs addons; where new features belong.
- **[Cache Contract](core-concepts/cache_contract.md)** — Keying, TTL, invalidation semantics (local-first + provider adapters).
- **[Observability Contract](core-concepts/observability_contract.md)** — Canonical logs/metrics/traces naming and provider adapters.
- **[Project Manifest Spec](core-concepts/project_manifest.md)** — Complete configuration schema for `project.yaml` (routing, databases, packaging, telemetry, and security).
- **[Resource Manifest Spec](core-concepts/resource_manifest.md)** — Complete configuration schema for `bffx/resources/*.yaml` (relationships, validations, and access controls).

---

## 📘 3. Topical Guides
*Deep-dives into specific framework layers:*
- **[Authentication & Session Security](guides/auth.md)** — JWT generation, guest sessions, device fingerprinting, and account linking.
- **[Outbound Communications & Alerts](guides/billing.md)** — Setting up email, SMS, and direct notification endpoints.
- **[Environment Configuration](guides/environment.md)** — Complete reference of all `BFFX_*` environment flags and secrets.
- **[Operations & Deployment](guides/operations.md)** — Structured logging, TLS termination, and reverse proxy configs.
- **[Production Runbook](operations/runbook.md)** — CLI ship commands, database backup routines, and Sentry configurations.
- **[Horizontal Scaling Checklist](operations/scaling.md)** — Multi-instance deployments checklist, Redis configurations, and cluster autoscaling.
- **[Caching Engine & Invalidation](guides/caching.md)** — In-memory vs Redis caching strategies and tagged invalidation semantics.
- **[Secret Security Inventory](guides/security_inventory.md)** — Hardening environment secrets and keys in staging and production.
- **[Action Authorization Contracts](guides/action_authorization.md)** — Sensitive name heuristic warnings and public routing auth controls.
- **[Framework Testing DSL](guides/testing.md)** — RSpec-style tests, blueprint dummy factories, and automated security audits.

---

## 📱 4. Client & Mobile Integration
*Connecting frontends to your Go orchestrator:*
- **[Flutter Mobile Integration](mobile/flutter.md)** — Auto-generated Dart SDK, models hydration, and state mapping.
- **[REST-Only Client](mobile/rest_client.md)** — Raw HTTP endpoints mapping, payload envelopes, and anonymous session handshakes.

---

## 📋 4b. Use cases
*End-to-end recipes (CLI commands + patterns) for common product shapes:*
- **[Use cases index](use-cases/README.md)**
- **[Headless CMS — 2026 WordPress alternative](use-cases/headless-cms-wordpress-alternative.md)** — Content API + admin; React/TS renders markdown (no server-side themes).

---

## 🔌 5. External Integrations
*Guides to connecting common SaaS stacks:*
- **[Stripe Payments](integrations/payments/stripe.md)** — Go SDK configuration, `BeforeCreate` intents, and secure webhook signature verification.
- **[Firebase Services](integrations/auth/firebase.md)** — Mobile push tokens mapping and HTTP v1 credentials.
- **[Appwrite Gateway Bridge](integrations/auth/appwrite.md)** — Proxying client identity via Appwrite session headers.
- **[Supabase Database](integrations/auth/supabase.md)** — Leveraging Supabase Postgres connection pooling and schema syncs.
- **[Amplitude Analytics](integrations/analytics/amplitude.md)** — Forwarding server-side actions events.
- **[Mixpanel Analytics](integrations/analytics/mixpanel.md)** — Bridging user behavioral analytics logs.
- **[PostHog Analytics](integrations/analytics/posthog.md)** — Native product event logging, batch processing, and self-hosted instances setup.

---

## 🔋 6. Built-in Batteries
*Configure pluggable system modules:*
- **[Batteries Overview](batteries/index.md)** — The pluggable architecture schema, defaults, and runtime overrides.
- **[PostgreSQL Database Store](batteries/postgres.md)** — Connecting to remote/managed SQL clusters and advisory schema locks.
- **[Feature Flags Engine](batteries/feature_flags.md)** — Local database-backed and LaunchDarkly flag evaluators.

---

## 🛠️ 7. Extending BFFX
*For advanced users customizing the core framework:*
- **[Custom Routing Hooks](extending/custom_routes.md)** — Registering custom Echo/Chi router handlers outside the manifest.
- **[Writing a Pluggable Battery](extending/writing_batteries.md)** — Scaffolding a custom battery provider satisfying a core framework interface.

---

## 📖 8. Lookup Tables
- **[CLI Command Reference](reference/cli.md)** — Directory execution matrix for all development, compilation, migration, and ship commands.

---

## 🤝 9. For Maintainers
- **[AI Directives](contributing/ai_directives.md)** — Model Context Protocol instructions and system prompts for coding agents.
- **[Test Coverage Gates](contributing/test_coverage.md)** — Framework telemetry unit and E2E integration test thresholds.

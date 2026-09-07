# BFFX Core Status & Feature Matrix

The following matrix outlines the production readiness, capability levels, and status of various core and addon modules in the BFFX framework.

| Module | Sub-component | Status | Maturity | Target |
|--------|---------------|--------|----------|--------|
| **Auth** | JWT / OTP / Revocation | Active | Production | v0.1.0 |
| **Storage** | CRUD / Migrations / Seeds | Active | Production | v0.1.0 |
| **Worker** | Cron / Background Queues | Active | Production | v0.1.0 |
| **AI Pipelines** | Chatbot (SSE + Rolling Memory) | Active | **Beta** | v0.1.0 |
| **AI Pipelines** | Ingestion (Sync / Async VLM) | Active | **Beta** | v0.1.0 |
| **SDUI** | Screen Builder / Layouts | Active | Beta | v0.1.0 |
| **Billing** | Stripe Integration Addon | Active | Production | v0.1.0 |
| **Archetypes** | Scaffold CLI / Presets registry | Active | **Beta** | v0.1.0 |

---

## AI Pipelines & Archetypes Maturity Notes
* **Chatbot (Beta)**: Fully supports multi-turn conversational agents with automated cache-backed rolling-session history. SSE token streaming verified.
* **Ingestion (Beta)**: Vision-guided multi-modal OCR catalog enrichment pipeline. Fully supports synchronous execution or asynchronous background job queueing.
* **Archetypes (Beta)**: High-speed vertical blueprints (`fintech`, `dictation`, `commerce`, `superapp`) with automated database mapping, async worker stubs, and pre-wired schemas.

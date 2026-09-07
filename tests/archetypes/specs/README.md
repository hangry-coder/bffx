# Archetype Validation Specifications Checklist

Use this checklist template to trace the Definition of Done (DoD) for each of the BFFX vertical archetypes across the test layers.

## Testing Matrix Overview

| Archetype Alias | L0 (Schema) | L1 (Scaffold) | L2 (Sync) | L3 (Build) | L4 (Contract) | L5 (Integration) | L5p (Pipeline) |
|---|---|---|---|---|---|---|---|
| **`superapp`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (chatbot) |
| **`fintech`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (ingestion async) |
| **`marketplace`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (agent stub) |
| **`microlearn`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (condenser stub) |
| **`dictation`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (ingestion async) |
| **`subscriptions`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (ingestion sync) |
| **`commerce`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (ingestion sync) |
| **`web3`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (rag stub) |
| **`iot`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] (agent stub) |
| **`notes`** | [ ] | [ ] | [ ] | [ ] | [ ] | [ ] | — |

---

## Definition of Done (DoD) Checklist

### 1. General Verification (All Archetypes)
- [ ] **L0 Schema:** Archetype YAML properties are structurally valid under registry definitions.
- [ ] **L1 Scaffold:** `bffx new <alias>` creates the exact expected directory layout and `project.yaml`.
- [ ] **L1 Anti-Drift Check:** No `batteries.nutrition` exists in `project.yaml` (nutrition utilizes pipeline-based catalog adapter).
- [ ] **L2 Sync:** Running `bffx sync` completes successfully without generating compiler warnings.
- [ ] **L3 Build:** Scaffolding builds and compiles (`go test -c ./cmd/api` or orchestrator works).
- [ ] **L4 Contract:** Required REST/OpenAPI endpoints exist (e.g., `/health`, `/api/v1/onboarding/...`).

### 2. Pipeline Integration (L5p) Verification
- [ ] **Ingestion Sync Pipeline:** `POST` route returns `200 OK` with mock VLM.
- [ ] **Ingestion Async Pipeline:** `POST` route returns `202 Accepted` and enqueues task in Worker queue.
- [ ] **Chatbot Pipeline:** `POST` with streaming returns SSE chunks (`text/event-stream` starting with `data:`).
- [ ] **Catalog Integration:** `catalog_source` is correctly populated if utilizing catalog adapters.
- [ ] **Mock Verification:** Ensures no live calls to Plaid, Stripe, or Groq are executed during `-short` CI runs.

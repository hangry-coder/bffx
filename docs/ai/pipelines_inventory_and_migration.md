---
title: AI Pipelines Inventory, Migration & Execution Flow
description: Comprehensive framework inventory, deprecation strategy for nutrition batteries, and the unified horizontal orchestrator execution flows.
category: architecture
---

# 📋 AI Pipelines: Inventory, Migration & Execution Flow

This document details the current-state architecture inventory, our modular migration plan for deprecating vertical dependencies, and the linear orchestrator execution model.

---

## 1. Current-State Abstraction Inventory

Before implementing any new pipeline components, we catalog the existing framework abstractions to ensure we **reuse existing libraries** rather than introducing duplicate definitions.

| Core Category | Abstraction Name | Current Location | Purpose & Reuse Strategy |
|:---|:---|:---|:---|
| **VLM & Models** | `vlm.Provider` | [`pkg/batteries/vlm/`](../../pkg/batteries/vlm) | Executes multimodal LLM requests. Already supports **Gemini**, **OpenRouter**, **Ollama**, and **noop** drivers. **Will be reused directly** by the pipeline orchestrator. |
| **Caching Engine** | `cache.Engine` | [`pkg/batteries/cache/`](../../pkg/batteries/cache) | Handles low-latency state. Already supports **in-memory** and **Redis** cache drivers. **Will be reused directly** to store chatbot history and vision optimization hashes. |
| **Streaming Ingress** | `streams.go` | [`pkg/api/handlers/streams.go`](../../pkg/api/handlers/streams.go) | Standard server-sent events (SSE) handler interface. We will model the Chatbot SSE stream closely on this to prevent writing a custom WebSocket layer. |
| **Background Jobs** | `Queue` & `Task` | [`pkg/worker/`](../../pkg/worker) | Standard Asynq-backed task runner. Used to queue heavy MLX and vision processing tasks when pipelines are flagged as `async: true`. |
| **Nutrition Integration** | `nutrition.Provider` | [`pkg/batteries/nutrition/`](../../pkg/batteries/nutrition) | *Deprecated:* Connects to Open Food Facts database. **To be demoted** to a domain addon under `pkg/addons/catalog/nutrition`. |

---

## 2. The Migration & Deprecation Plan

To maintain backwards compatibility for existing deployments (such as reference calorie estimation services), we will implement a phased deprecation path for the vertical nutrition battery:

### Phase A: The Compatibility Shim (v0.1.0-beta)
- Mark `spec.batteries.nutrition` as **deprecated** in the OpenAPI schema and linter.
- If a project manifest retains `nutrition: openfoodfacts`, the framework will successfully load it, but `bffx doctor` will print a prominent warning:
  > ⚠️ **Warning**: `batteries.nutrition` is deprecated and will be removed in v0.2.0. Please migrate to declarative Ingestion Pipelines utilizing the `openfoodfacts` catalog adapter.

### Phase B: Full Removal (v0.2.0)
- Completely delete the `pkg/batteries/nutrition` directory.
- Relocate all logic exclusively to `pkg/addons/catalog/nutrition`.
- Raise a hard compilation error if a project manifest retains the deprecated `nutrition` key.

---

## 3. Unified Ingestion Execution Flow

The linear ingestion pipeline honors strict execution boundaries, ensuring media assets are optimized, cached, evaluated against models, resolved through catalogs, and persisted with zero redundant actions.

```mermaid
sequenceDiagram
    autonumber
    actor Client as Flutter / API Client
    participant API as API Ingress (Mux)
    participant Orch as IngestionEngine (Orchestrator)
    participant Opt as Media Optimizer
    participant Cache as Cache Battery
    participant VLM as VLM Battery Cascade
    participant Cat as Catalog Adapter

    Client->>API: POST /api/v1/meals/scan (Image + Headers)
    API->>Orch: Invoke Pipeline Handler
    Orch->>Opt: Process(MIME Sniff, Resize, SHA-256)
    Opt-->>Orch: Return MediaAsset (Hash, Optimized Bytes)

    alt Cache Check (Bypass Header ABSENT)
        Orch->>Cache: Get(MediaAsset.Hash)
        Cache-->>Orch: Found (Return cached JSON)
        Orch-->>Client: Respond 200 OK (Cache Hit)
    else Cache Miss OR Cache Bypass Header PRESENT
        Orch->>VLM: Cascade Model Routing (Gemini -> OpenRouter)
        Note over VLM: If Model A fails (Quota), Fallback to Model B
        VLM-->>Orch: Return Structured JSON text
        Orch->>Cat: Resolve(Structured JSON hints)
        Cat-->>Orch: Hydrated Domain Records
        Orch->>Cache: Set(MediaAsset.Hash, TTL)
        Orch-->>Client: Respond 200 OK (Fresh Ingest)
    end
```

### Cache Bypass Strategy (Audit I-05)
By default, identical media uploads are intercepted by the cache layer using the generated SHA-256 hash. To bypass the cache during development or manual overrides:
1. **API Header**: Clients can send the HTTP header `X-Cache-Bypass: true`.
2. **Manifest Flag**: The pipeline configuration can declare `settings.cache_bypass: true` to force live VLM execution on every request.

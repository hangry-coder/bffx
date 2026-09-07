---
title: Observability Contract (Phase 4)
description: Canonical telemetry semantics and provider adapter rules.
category: core-concepts
---

# Observability Contract (Phase 4)

This document defines the canonical observability contract for BFFX runtime code and provider adapters.

## 1) Two-Layer Model

- **Core telemetry semantics layer**
  - HTTP/gRPC correlation headers, structured log field keys, Prometheus metric names, audit entry shapes.
  - Owned by `pkg/observability` (`contract.go`, `correlation.go`, `audit.go`, `record.go`).
- **Provider adapter layer**
  - Log shipping backends (`pkg/batteries/observability`: slog/axiom/sentry).
  - Admin dashboard read models (`pkg/observability/provider.go`: incidents/telemetry/analytics).

Local-first rule: when external credentials are missing, adapters fall back to local slog and built-in store-backed providers.

## 2) Canonical Correlation Headers

Defined in `pkg/observability/contract.go`:

- `X-Trace-ID` — primary trace/correlation identifier.
- `X-Correlation-ID` — client alias; mirrors `X-Trace-ID` on responses.
- `X-Request-ID` — per-request identifier (defaults to trace ID when absent).
- `X-B3-TraceId` — legacy Zipkin compatibility on ingress only.

gRPC metadata equivalents: `x-trace-id`, `x-correlation-id`, `x-request-id`, `traceparent`.

Helpers:

- `observability.ResolveIncomingTraceID(...)`
- `observability.ResolveIncomingRequestID(...)`
- `observability.ApplyCorrelationHeaders(...)`

## 3) Canonical Log Fields

- `trace_id`
- `request_id`
- `correlation_id` (reserved for structured payloads; HTTP header mirrors trace ID)

Access logs, slog adapters, and API error payloads must use these keys via `observability.Field*` constants.

Note: `pkg/logger` uses matching literal keys (`trace_id`, `request_id`) to avoid an import cycle with `pkg/observability` (which transitively imports storage).

## 4) Canonical Prometheus Metrics

All metrics use the `bffx_` prefix:

- `bffx_http_requests_total` — labels: `method`, `path`, `status`
- `bffx_http_request_duration_seconds` — labels: `method`, `path`
- `bffx_auth_attempts_total`
- `bffx_registry_resources_total`
- `bffx_worker_jobs_total`
- `bffx_worker_job_duration_seconds`

Runtime recording helper: `observability.RecordHTTPRequest(...)`.

## 5) Log Provider Battery (`batteries.observability`)

Canonical manifest values:

- `slog` (default, local-first)
- `axiom`
- `sentry`

Interface: `batteries/observability.LogProvider` (alias of `Provider`).

Deprecated env selectors for admin dashboard providers (incidents/telemetry/analytics):

- `BFFX_INCIDENT_PROVIDER`
- `BFFX_TELEMETRY_PROVIDER`
- `BFFX_ANALYTICS_PROVIDER`

Prefer manifest batteries configuration; doctor warns when legacy env overrides are set.

## 6) Audit Contract Alignment

Canonical runtime audit types live in `pkg/observability/audit.go`:

- `observability.AuditEntry`
- `observability.AuditLevel*`
- `observability.AuditProvider` (alias of `batteries/audit.Provider`)

Legacy admin storage shape: `pkg/audit.AuditLog` with `ToEntry()` conversion helper.

Runtime router audit writes use `observability.AuditEntry`; admin panel continues persisting `AuditLog` resources.

## 7) Phase 4 Usage Map (Primary Paths)

- Core contracts: `pkg/observability/contract.go`, `correlation.go`, `audit.go`, `record.go`.
- HTTP middleware: `pkg/api/middleware/middleware.go`, `pkg/api/middleware/metrics.go`.
- Log adapters: `pkg/batteries/observability/*`.
- gRPC correlation: `pkg/batteries/wire/grpc/interceptors.go`.
- Doctor lint: `lintObservabilityContract` in `pkg/doctor/doctor.go`.

## 8) Deprecation Timeline

- **Now → next minor:** legacy env selectors and duplicate string literals emit doctor warnings when touched paths are validated.
- **Next minor → following minor:** new code must import canonical constants/helpers; ad-hoc metric/header names are rejected in doctor for generated projects.
- **Following minor+:** remove legacy env selectors after pilot migration gates pass.

## 9) Related docs

- [Architecture contracts](architecture_contracts.md) — program index
- [Build profile contract](build_profile_contract.md) — runtime capability surfaces
- [Operations runbook](../operations/runbook.md) — production observability wiring

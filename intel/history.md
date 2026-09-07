# Intel history

## 2026-09-06 — #cursor (open source core readiness)

- **Dependency modernization:** Upgraded core Go packages (`golang.org/x/crypto v0.56.0`, `golang.org/x/net v0.58.0`, `golang.org/x/sync v0.22.0`, `github.com/redis/go-redis/v9 v9.22.0`, `modernc.org/sqlite v1.58.0`, `go.opentelemetry.io/otel v1.46.0`).
- **CLI UX fix:** Handled `help`, `--help`, and `-h` commands in `cmd/bffx/main.go` with standard exit code 0.
- **CI & repository decoupling:** Removed external pilot contract smoke tests from CI and decoupled tests from external projects.
- **Repository isolation:** Preserved and isolated proprietary pilot assets into local gitignored storage; sanitized all framework docs and tests.
- **Test verification:** Ran full unit and smoke test suites across `pkg/...` and `tests/...` (100% passing). #cursor

## 2026-06-20 — #cursor (release readiness audit)

- **Framework audit:** Completed release readiness evaluation — `docs/release-readiness/BFFX_CORE_AUDIT.md`. Covers security, privacy, data, scaling, testing, ops. **Verdict:** BFFX core ready for OSS v0.1.x beta with documented guardrails. #cursor

## 2026-06-03 — #cursor (framework hardening & wave 6)

- **Wave 6 / consolidated plan:** `TestProductionCode_DoesNotImportAdmin`; router tests use `runtimecontracts` only; `architecture-baseline.sh` reports `.bffx/core` size; CI runs baseline on PR; ownership map in `architecture_contracts.md`.
- **CI:** Bumped `go` / GitHub Actions to **1.26.4** for govulncheck stdlib fixes (GO-2026-5039, GO-2026-5037).
- **Framework generator DX (Wave 2c):** Screen CLI passes `Group`/`Layout`; v2 `AdminResource` → `manifests/resources/`; Flutter auto-`protoc` (warn-only); `kind: Seed` + `SeedRegistry` + `bffx seed` via `NewStore`.
- **Pre-release framework gates:** `internal/features/*/seeds/` in v2 manifest walk; `SeedSpec.order`/`upsert`, seed sort, `bffx seed --upsert`, name lookup + `Update` upsert path; `bffx doctor` seed manifest hint; reject `MOCK_TOKEN_*` when `BFFX_ENV=production`. Tests: `pkg/manifest`, `pkg/storage`, `pkg/auth`, `pkg/doctor`. #cursor
- **Wave 0 (consolidated plan):** v2 example smokes (`cmd/api`), `examples/notes` v2 layout migration, architecture-baseline entry detection, CI `example-modules` job, smoke/e2e/watch path fixes.
- **Wave 1:** `schema.MigrationsAdopted` gate in `compiler/sync` (Track B), `bffx doctor` migration baseline warn.
- **Wave 2:** Removed public `/api/docs` auth bypass; admin sidebar driven by `AdminSite.spec.menu` (`finalizeAdminMenu` + `AdminNav`); docs updated for v2 manifest paths and admin-only API reference.
- **Wave 2b:** Split `pkg/admin/handlers/resources.go` into CRUD + helpers/config/actions/export; extracted admin UI shared components under `pkg/admin/ui-v2/src/components/`; rebuilt `dist/`.
- **Admin relations/menu/session:** Full default AdminSite menu + session block in generator; v2 session cookie + idle logout; belongs_to/associations + GET associations API; RelationLink/AssociationPanel UI.
- **OSS launch waves 3–5:** Refresh tokens hashed (`rt1.{id}.{secret}`), login/signup lockout middleware, Google/Apple `aud` env validation, Clerk HMAC fallback disabled in prod, `security_inventory.md` expanded, deploy Dockerfile go.mod cache layer, deploy CLI help + deployment README command matrix. #cursor
- **Docs:** Added `docs/use-cases/` with headless CMS (2026 WordPress alternative) recipe — CLI command cheat sheet, manifest phases, React/markdown frontend; linked from `docs/README.md`. #cursor

# Coverage gates (PR CI)

PR CI runs `go test -short -coverprofile=coverage.out ./pkg/... ./tests/cli/... ./tests/smoke/...` then `go run tools/coverage_gate.go coverage.out`.

## Package floors

Floors are defined in [`tools/coverage_gate.go`](../../tools/coverage_gate.go) as `defaultPackages`. Current floors (ratchet upward only with maintainer agreement and test additions):

| Package prefix | Floor |
|----------------|-------|
| `bffx/pkg/api/handlers` | 50% |
| `bffx/pkg/api/middleware` | 50% |
| `bffx/pkg/auth` | 55% |
| `bffx/pkg/storage` | 43% |
| `bffx/pkg/api/router` | 50% |

**Missing package** in the profile (no statements attributed to that import path) fails the gate — ensure tests touch each listed tree or adjust the allowlist deliberately.

## Self-test

`go test ./tools/...` exercises the profile parser and pass/fail logic in `coverage_gate_internal_test.go`.

## Full-stack e2e (generated app)

`TestFullStackIntegration` in `tests/e2e/` scaffolds a disposable app under `test-projects/`, runs the **bffx dev** build path (`go build ./cmd/api`), and asserts:

- `cmd/api/main.go` wires `ActionHandlers` / `HookHandlers` (catches custom-action 404 regressions)
- REST CRUD + custom action over HTTP
- gRPC `ActionService` + `NoteService` (requires `buf` + `protoc-gen-go-grpc` in PATH)
- Admin SPA + `/admin/api/admin/*` session APIs
- SQLite persistence across server restart

```bash
go test ./tests/e2e/ -run TestFullStackIntegration -v -count=1
```

JSON step reports land in `tests/e2e/logs/full_stack_*.json`; server stdout/stderr in `tests/e2e/logs/e2e-full-stack_*_server.log`. The test app is always deleted at the end.

Optional Postgres: `E2E_STORE=postgres E2E_DATABASE_URL=postgres://... go test ...`

## Updating floors

1. Run the same `go test -coverprofile` command locally.
2. Inspect per-package percentages in the gate output.
3. If raising a floor, add tests in the same PR; do not raise floors without coverage to match.

# Contributing to BFFX

Thank you for your interest in contributing to BFFX! As a production-grade framework, we maintain high standards for code quality, testing, and documentation.

## Development Principles

1.  **Idiomatic Go**: Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
2.  **Context Propagation**: Every function that performs I/O or calls a battery MUST accept `context.Context` as its first argument.
3.  **Interface Segregation**: Keep battery interfaces lean. New features should be added as new batteries or optional interface methods.
4.  **No Global State**: Dependencies must be injected via constructors. Avoid `init()` functions that set up global variables.

## Getting Started

### Prerequisites
*   Go 1.26+
*   Redis (for cache/auth revocation tests)
*   Docker (optional, for running full integration suites)

### Workspace Hygiene & Multi-Module Setup
BFFX is structured as a multi-module repository to ensure clean test boundaries and standalone compilation guarantees.
- **No Global `go.work`**: To prevent local path leaks and private project conflicts, the root `go.work` is ignored by Git.
- **Workspace Scaffolding**: To work across multiple packages locally with IDE auto-imports, you can copy our example configuration:
  ```bash
  cp go.work.example go.work
  ```
- **Parity Building**: When compiling or running tests, the CLI leverages `GOWORK=off` internally to isolate main modules.

### Running Tests
We use standard Go testing. Ensure all tests pass in short mode and full integration parity prior to opening a PR.

**Run short unit tests:**
```bash
go test ./... -short
```

**Run telemetry-specific tests:**
```bash
go test -tags=sentry ./pkg/observability/...
```

**Run integration and E2E suites (requires Docker parity stack):**
```bash
docker compose -f docker-compose.test.yml up -d
DATABASE_URL="postgres://bffx:bffx@127.0.0.1:5433/bffx?sslmode=disable" go test -tags=integration ./tests/integration/...
```

### Adding a New Battery
1.  Define the interface in `pkg/batteries/<name>/provider.go`.
2.  Provide a default/mock implementation for development.
3.  Update `RouterConfig` and `Router` in `pkg/api/router/router.go` to include the new battery.
4.  Add unit tests for the battery logic.

## Pull Request Process

1.  **Issue First**: Please open an issue to discuss significant changes before starting work.
2.  **Tests Required**: Every bug fix or new feature must include corresponding tests.
3.  **Documentation**: Update `ARCHITECTURE.md` or the relevant `pkg/` docstrings if the design changes.
4.  **Linters**: Run `go vet` and `golangci-lint` (if available) to ensure code style consistency.

## Testing Standards
*   **Unit Tests**: Required for all new packages and batteries.
*   **Integration Tests**: Required for changes to the `Router` or `Store` layers.
*   **Benchmarks**: Encouraged for performance-critical paths (e.g., Caching, JSON serialization).

### CI/CD Job Matrix & Test Scopes

Every Pull Request and commit to the `main` branch triggers our GitHub Actions CI pipeline. The following table maps each CI job to its corresponding test scope and execution criteria:

| CI Job Name | Trigger Events | Scope / Command | Purpose |
|:---|:---|:---|:---|
| **Smoke Tests (Fast)** | Push, PR, Schedule | `go test -v -short ./...` | Runs the full suite of unit, cli, golden, and helper tests in short mode (no live DBs required). Checks a 70%+ coverage gate. |
| **Sentry Tag Build** | Push, PR, Schedule | `go test -tags=sentry ./pkg/observability/...` | Validates compilation and core assertions when Sentry integrations are enabled. |
| **Lint** | Push, PR, Schedule | `golangci-lint run` | Enforces Uber Go style rules, `revive` exported comments, and static code hygiene. |
| **Vulnerability Scan** | Push, PR, Schedule | `govulncheck-action` | Scans the dependency graph for known Go package vulnerabilities. |
| **Race Detection** | Push, PR, Schedule | `go test -race -count=1 -timeout=10m ./pkg/... ./tests/api/... ./tests/cli/... ./tests/golden/... ./tests/smoke/...` | Validates concurrency security and identifies data races across all packages. |
| **Integration** | Push, PR, Schedule | `go test -tags=integration ./tests/integration/...` | Spins up a temporary Postgres/Redis docker-compose stack and runs database/caching integration tests. |
| **Experimental Storage** | Push, PR, Schedule | `go build -tags=experimental_mongo,experimental_pocketbase ./pkg/storage/...` | Ensures the experimental database engines compile successfully. |
| **Full E2E** | Nightly, Manual | `go test -v ./tests/acceptance/...` and `./tests/e2e/...` | Runs full application E2E test suites (Docker compilation, scaffolding, CLI operations). |

## Security
If you find a security vulnerability, please do not open a public issue. Instead, contact the maintainers directly.


# End-to-end tests (`tests/e2e`)

## `bffx` binary

These tests **do not** assume a checked-in `bffx` binary at the repo root.  
`bffx_cli.go` discovers the module root (walk upward until `go.mod` contains `module bffx`), then runs **`go build -o $TMPDIR/bffx-e2e-<pid> ./cmd/bffx`** once per process and reuses that path.

Scaffolding commands should pass **`--non-interactive`** so the CLI wizard does not read stdin.

## Modes

- **`go test -short ./tests/e2e/...`:** Runs fast checks (including deploy/CICD output tests that shell out to the built CLI). Heavy persona / security tests skip in short mode.
- **Full e2e:** Omit `-short` and set env as required by individual tests (see `common_test.go`).

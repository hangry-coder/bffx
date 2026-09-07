package e2e

import "testing"

// TestFullStackIntegration scaffolds a disposable app, runs it via the bffx dev
// build path (go build ./cmd/api), and asserts REST + gRPC + admin + DB persistence.
//
// Reports are written to tests/e2e/logs/full_stack_*.json; server stdout/stderr
// goes to tests/e2e/logs/e2e-full-stack_*_server.log. The generated app under
// test-projects/ is always deleted at the end.
//
// Run: go test ./tests/e2e/ -run TestFullStackIntegration -v -count=1
// Optional: E2E_STORE=postgres E2E_DATABASE_URL=postgres://... for Postgres.
func TestFullStackIntegration(t *testing.T) {
	runFullStackHarness(t)
}

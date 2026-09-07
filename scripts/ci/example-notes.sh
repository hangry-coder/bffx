#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# Fulfilling P0 task 0.1.4: Install via go install
go install ./cmd/bffx
export PATH="$(go env GOPATH)/bin:$PATH"

cd "${ROOT}/examples/notes"
export GOWORK=off
bffx sync
go mod tidy
go test ./...

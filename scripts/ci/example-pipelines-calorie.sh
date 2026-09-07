#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

go install ./cmd/bffx
export PATH="$(go env GOPATH)/bin:$PATH"

cd "${ROOT}/examples/pipelines-calorie"
export GOWORK=off
bffx sync
go mod tidy
go test ./... -short

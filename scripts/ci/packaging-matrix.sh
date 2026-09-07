#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

echo "== Packaging matrix (full vs minimal) =="

go test -count=1 ./pkg/buildprofile -run 'TestPackagingMigration|TestPackagingMigrationMatrix' -timeout=5m

mkdir -p "${ROOT}/.bffx/bin"
GOBIN="${ROOT}/.bffx/bin" go install ./cmd/bffx
export PATH="${ROOT}/.bffx/bin:$PATH"
export GOWORK=off

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ms_now() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

benchmark_packaging() {
  local name="$1"
  local minimal_flag="$2"
  local app="${name}-app"
  local app_root="${TMP}/${app}"

  if [[ "$minimal_flag" == "true" ]]; then
    bffx new "$app" --minimal --root "$TMP" --non-interactive >/dev/null
  else
    bffx new "$app" --root "$TMP" --non-interactive >/dev/null
  fi

  pushd "$app_root" >/dev/null
  # bffx new may vendor .bffx/core and add replace => ./.bffx/core; point at monorepo for CI.
  go mod edit -require=github.com/hangry-coder/bffx@v0.0.0
  go mod edit -replace=github.com/hangry-coder/bffx="${ROOT}"
  go mod tidy

  local sync_start sync_end sync_ms
  sync_start="$(ms_now)"
  bffx sync >/dev/null
  sync_end="$(ms_now)"
  sync_ms="$((sync_end - sync_start))"

  if [[ ! -f .bffx/build-profile.json ]]; then
    echo "FAIL: ${name} missing build-profile.json after sync" >&2
    exit 1
  fi

  local mode
  mode="$(python3 - <<'PY' "$app_root"
import json, sys
print(json.load(open(sys.argv[1]+"/.bffx/build-profile.json"))["mode"])
PY
)"

  local build_start build_end build_ms
  build_start="$(ms_now)"
  local entry="./cmd/api"
  if [[ ! -d cmd/api ]]; then
    entry="./cmd/orchestrator"
  fi
  go build -o .bffx/orchestrator.packaging "${entry}"
  build_end="$(ms_now)"
  build_ms="$((build_end - build_start))"
  local bin_size
  bin_size="$(wc -c < .bffx/orchestrator.packaging | tr -d ' ')"

  echo "${name} mode=${mode} sync_ms=${sync_ms} build_ms=${build_ms} bin_bytes=${bin_size}"
  popd >/dev/null

  # Soft gate: sync should complete within 120s on CI runners
  if [[ "$sync_ms" -gt 120000 ]]; then
    echo "FAIL: ${name} sync_ms=${sync_ms} exceeds 120000ms threshold" >&2
    exit 1
  fi
}

benchmark_packaging "full" "false"
benchmark_packaging "minimal" "true"

echo "Packaging matrix passed."

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UI_DIR="${ROOT}/pkg/admin/ui-v2"

if [[ ! -f "${UI_DIR}/package.json" ]]; then
  echo "admin UI not found at ${UI_DIR}" >&2
  exit 1
fi

cd "${UI_DIR}"
if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required to build admin UI (pkg/admin/ui-v2/dist)" >&2
  exit 1
fi

npm ci
npm run build

if [[ ! -f "${UI_DIR}/dist/index.html" ]]; then
  echo "build did not produce dist/index.html" >&2
  exit 1
fi

echo "Admin UI built: ${UI_DIR}/dist"

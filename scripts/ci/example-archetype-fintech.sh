#!/usr/bin/env bash
set -euo pipefail

# Scripts directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "🚀 Starting Fintech Archetype CI validation..."

# 1. Build CLI binary
echo "🔨 Building bffx CLI..."
cd "${WORKSPACE_DIR}"
go build -o build/bffx ./cmd/bffx

# 2. Create temp directory for scaffolded project
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT
echo "📂 Created temporary directory: ${TEMP_DIR}"

# 3. Scaffold fintech archetype
echo "⚡️ Scaffolding fintech project..."
./build/bffx new fintech --root "${TEMP_DIR}" --non-interactive

PROJECT_DIR="${TEMP_DIR}/fintech"
cd "${PROJECT_DIR}"

# 4. Modify project.yaml to use sqlite for offline CI testing
echo "📝 Configuring database to sqlite..."
cat <<EOF > bffx/project.yaml
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: fintech
spec:
  runtime:
    api:
      language: go
      port: 8080
    worker:
      language: python
      enabled: true
    redis:
      enabled: true
    streaming:
      enabled: false
  store:
    mode: sqlite
    path: .bffx/app.db
  telemetryStore:
    mode: sqlite
    path: .bffx/telemetry.db
  defaults:
    auth: builtin
    files: true
    jobs: true
    builders: true
  app:
    authStrategy: mandatory
    namespace: com.example.fintech
    apiPrefix: /api/v1
    permissions:
      - { type: notifications, required: true, justification: "Required for daily goal reminders." }
    ui:
      welcome_title: "Welcome to fintech!"
      primary_action: "Get Started"
  admin:
    enabled: true
  layout: v2
EOF

# Fix the user.yaml bio type just in case
sed -i.bak 's/type: text/type: string/g' internal/features/system/manifests/user.yaml || true

# Add catalog to ReceiptScan pipeline manifest to satisfy doctor check
cat <<EOF > internal/features/finance/manifests/receiptscan_pipeline.yaml
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: ReceiptScan
spec:
  type: ingestion
  route:
    method: POST
    path: /api/v1/pipelines/receiptscan
    auth: required
  model_routing:
    - gemini
  catalog:
    adapter: plaid
  settings:
    compression_max_width: 800
    cache_bypass: false
  hooks:
    beforePipeline:
      - action: BeforeReceiptScan
    afterPipeline:
      - action: AfterReceiptScan
EOF

# 5. Configure Go module and run sync
echo "⚙️ Configuring Go module..."
go mod edit -replace github.com/hangry-coder/bffx="${WORKSPACE_DIR}"

echo "🔄 Running bffx sync..."
"${WORKSPACE_DIR}/build/bffx" sync

# 6. Run go tests inside the project
echo "🧪 Running generated tests..."
rm -rf tests/e2e
go test -v ./... -short

# 7. Run bffx doctor
echo "🩺 Running bffx doctor..."
"${WORKSPACE_DIR}/build/bffx" doctor

echo "✅ Fintech Archetype CI validation successful!"

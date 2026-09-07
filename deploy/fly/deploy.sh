#!/usr/bin/env bash
set -euo pipefail

echo "==> Deploying BFFX to Fly.io..."
if ! command -v fly &> /dev/null && ! command -v flyctl &> /dev/null; then
    echo "Error: fly CLI is not installed. Visit https://fly.io/docs/hands-on/install-flyctl/"
    exit 1
fi

FLY_CMD=$(command -v fly || command -v flyctl)

# Check if app exists or launch
if [ ! -f "fly.toml" ]; then
    cp deploy/fly/fly.toml ./fly.toml
fi

# Create persistent volume if not existing
if ! $FLY_CMD volumes list 2>/dev/null | grep -q bffx_data; then
    echo "Creating persistent storage volume 'bffx_data' (1GB)..."
    $FLY_CMD volumes create bffx_data --region iad --size 1 -y
fi

$FLY_CMD deploy
echo "==> BFFX successfully deployed to Fly.io!"

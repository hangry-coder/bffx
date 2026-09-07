#!/bin/bash
set -e

# BFFX Release Script
# Automates version bumping and tagging

if [ -z "$1" ]; then
    echo "Usage: ./scripts/release.sh <vX.Y.Z>"
    exit 1
fi

VERSION=$1

# 1. Update internal version file
echo "Updating pkg/version/version.go to $VERSION..."
cat > pkg/version/version.go <<EOF
package version

// FrameworkVersion is the current version of the BFFX framework.
// This is used for CLI display and telemetry.
const FrameworkVersion = "$VERSION"
EOF

# 2. Update CHANGELOG.md (placeholder check)
if ! grep -q "$VERSION" CHANGELOG.md; then
    echo "⚠️  $VERSION not found in CHANGELOG.md. Please update it first."
    exit 1
fi

# 3. Run tests one last time
echo "Running smoke tests..."
go test -v -short ./tests/smoke/...

# 4. Commit version bump
git add pkg/version/version.go
git commit -m "chore: bump version to $VERSION"

# 5. Tag
echo "Tagging $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "✨ Release $VERSION prepared locally."
echo "Next steps:"
echo "1. git push origin main"
echo "2. git push origin $VERSION"
echo "3. GoReleaser will take it from there in CI."

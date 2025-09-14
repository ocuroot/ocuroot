#!/bin/bash

# Build script for creating distribution packages for all architectures
set -e

VERSION=${1:-"1.0.0"}
DIST_DIR="$(dirname "$0")"

echo "Building distribution packages for version: $VERSION"

# Build packages for both architectures
"$DIST_DIR/build-package.sh" amd64 "$VERSION"
"$DIST_DIR/build-package.sh" arm64 "$VERSION"

echo ""
echo "🎉 All packages built successfully!"

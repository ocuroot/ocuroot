#!/bin/bash

# Test script for verifying a single architecture package installation
set -e

ARCH=${1}
VERSION=${2:-"1.0.0"}

if [ -z "$ARCH" ]; then
    echo "Usage: $0 <architecture> [version]"
    echo "  architecture: amd64 or arm64"
    echo "  version: package version (default: 1.0.0)"
    exit 1
fi

if [ "$ARCH" != "amd64" ] && [ "$ARCH" != "arm64" ]; then
    echo "❌ Unsupported architecture: $ARCH"
    echo "Supported architectures: amd64, arm64"
    exit 1
fi

TEST_DIR="$(dirname "$0")"
DIST_DIR="$(dirname "$TEST_DIR")"
ROOT_DIR="$(dirname "$DIST_DIR")"

echo "Testing ocuroot $ARCH package for version: $VERSION"

# Check if package exists
PACKAGES_DIR="$ROOT_DIR/.build/packages"
PACKAGE_DEB="$PACKAGES_DIR/ocuroot_${VERSION}_${ARCH}.deb"

if [ ! -d "$PACKAGES_DIR" ]; then
    echo "❌ Packages directory not found: $PACKAGES_DIR"
    echo "Run ./distribution/build-package.sh $ARCH first"
    exit 1
fi

if [ ! -f "$PACKAGE_DEB" ]; then
    echo "❌ $ARCH package not found: $PACKAGE_DEB"
    echo "Run ./distribution/build-package.sh $ARCH first"
    exit 1
fi

echo "✓ Found $ARCH package"

# Test package
echo ""
echo "=== Testing $(echo $ARCH | tr '[:lower:]' '[:upper:]') Package ==="
cd "$ROOT_DIR"
docker build --platform linux/${ARCH} --build-arg VERSION=${VERSION} -f distribution/test/Dockerfile.${ARCH} -t ocuroot-test-${ARCH} . || {
    echo "❌ $ARCH package test failed"
    exit 1
}
echo "✓ $ARCH package test passed"

echo ""
echo "🎉 $ARCH package test passed!"
echo ""
echo "Available test image:"
echo "  - ocuroot-test-${ARCH} (for $ARCH testing)"
echo ""
echo "To run interactive test:"
echo "  docker run -it --rm ocuroot-test-${ARCH}"

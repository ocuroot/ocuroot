#!/bin/bash

# Test script for verifying package installations across architectures
set -e

VERSION=${1:-"1.0.0"}
TEST_DIR="$(dirname "$0")"
DIST_DIR="$(dirname "$TEST_DIR")"
ROOT_DIR="$(dirname "$DIST_DIR")"

echo "Testing ocuroot packages for version: $VERSION"

# Check if packages exist
PACKAGES_DIR="$ROOT_DIR/.build/packages"
if [ ! -d "$PACKAGES_DIR" ]; then
    echo "❌ Packages directory not found: $PACKAGES_DIR"
    echo "Run ./distribution/build-packages.sh first"
    exit 1
fi

AMD64_DEB="$PACKAGES_DIR/ocuroot_${VERSION}_amd64.deb"
ARM64_DEB="$PACKAGES_DIR/ocuroot_${VERSION}_arm64.deb"

if [ ! -f "$AMD64_DEB" ]; then
    echo "❌ AMD64 package not found: $AMD64_DEB"
    exit 1
fi

if [ ! -f "$ARM64_DEB" ]; then
    echo "❌ ARM64 package not found: $ARM64_DEB"
    exit 1
fi

echo "✓ Found required packages"

# Test AMD64 package
echo ""
echo "=== Testing AMD64 Package ==="
cd "$ROOT_DIR"
docker build --platform linux/amd64 -f distribution/test/Dockerfile.amd64 -t ocuroot-test-amd64 . || {
    echo "❌ AMD64 package test failed"
    exit 1
}
echo "✓ AMD64 package test passed"

# Test ARM64 package
echo ""
echo "=== Testing ARM64 Package ==="
docker build --platform linux/arm64 -f distribution/test/Dockerfile.arm64 -t ocuroot-test-arm64 . || {
    echo "❌ ARM64 package test failed"
    exit 1
}
echo "✓ ARM64 package test passed"

echo ""
echo "🎉 All package tests passed!"
echo ""
echo "Available test images:"
echo "  - ocuroot-test-amd64 (for AMD64 testing)"
echo "  - ocuroot-test-arm64 (for ARM64 testing)"
echo ""
echo "To run interactive tests:"
echo "  docker run -it --rm ocuroot-test-amd64"
echo "  docker run -it --rm ocuroot-test-arm64"

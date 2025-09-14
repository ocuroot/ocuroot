#!/bin/bash

# Test script for verifying package installations across architectures
set -e

VERSION=${1:-"1.0.0"}
TEST_DIR="$(dirname "$0")"

echo "Testing ocuroot packages for version: $VERSION"

# Test packages for both architectures
"$TEST_DIR/test-package.sh" amd64 "$VERSION"
"$TEST_DIR/test-package.sh" arm64 "$VERSION"

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

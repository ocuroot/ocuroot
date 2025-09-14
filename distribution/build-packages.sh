#!/bin/bash

# Build script for creating distribution packages
set -e

VERSION=${1:-"1.0.0"}
DIST_DIR="$(dirname "$0")"
ROOT_DIR="$(dirname "$DIST_DIR")"

echo "Building distribution packages for version: $VERSION"

# Create distribution output directory under .build
BUILD_PACKAGES_DIR="$ROOT_DIR/.build/packages"
mkdir -p "$BUILD_PACKAGES_DIR"

# Build AMD64 package
echo "Building AMD64 package..."
pushd "$DIST_DIR" > /dev/null
nfpm pkg --config nfpm-amd64.yaml --packager deb --target "../.build/packages/ocuroot_${VERSION}_amd64.deb"
nfpm pkg --config nfpm-amd64.yaml --packager rpm --target "../.build/packages/ocuroot_${VERSION}_amd64.rpm"

# Build ARM64 package
echo "Building ARM64 package..."
nfpm pkg --config nfpm-arm64.yaml --packager deb --target "../.build/packages/ocuroot_${VERSION}_arm64.deb"
nfpm pkg --config nfpm-arm64.yaml --packager rpm --target "../.build/packages/ocuroot_${VERSION}_arm64.rpm"
popd > /dev/null

echo "Package build complete!"
echo "Packages created in: $BUILD_PACKAGES_DIR/"
ls -la "$BUILD_PACKAGES_DIR/"

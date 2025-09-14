#!/bin/bash

# Build script for creating a single architecture package
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

DIST_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$DIST_DIR")"

echo "Building $ARCH package for version: $VERSION"

# Create distribution output directory under .build
BUILD_PACKAGES_DIR="$ROOT_DIR/.build/packages"
mkdir -p "$BUILD_PACKAGES_DIR"

# Build package for specified architecture
echo "Building $ARCH package..."

# Create temporary config with version substitution in .build
TEMP_CONFIG_DIR="$ROOT_DIR/.build/nfpm-configs"
mkdir -p "$TEMP_CONFIG_DIR"
TEMP_CONFIG="$TEMP_CONFIG_DIR/nfpm-${ARCH}-${VERSION}.yaml"
sed "s/{{VERSION}}/${VERSION}/g" "$DIST_DIR/nfpm-${ARCH}.yaml" > "$TEMP_CONFIG"

pushd "$DIST_DIR" > /dev/null
nfpm pkg --config "$TEMP_CONFIG" --packager deb --target "../.build/packages/ocuroot_${VERSION}_${ARCH}.deb"
nfpm pkg --config "$TEMP_CONFIG" --packager rpm --target "../.build/packages/ocuroot_${VERSION}_${ARCH}.rpm"
popd > /dev/null

# Clean up temporary config
rm "$TEMP_CONFIG"

echo "Package build complete!"
echo "Packages created:"
echo "  - $BUILD_PACKAGES_DIR/ocuroot_${VERSION}_${ARCH}.deb"
echo "  - $BUILD_PACKAGES_DIR/ocuroot_${VERSION}_${ARCH}.rpm"

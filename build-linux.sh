#!/bin/bash

set -e

# Simple Linux build script for tailstream-agent

PROJECT_NAME="tailstream-agent"
BUILD_DIR="dist"

echo "Building $PROJECT_NAME for Linux..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Build for Linux x64
cd agent
echo "Building for linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o "../$BUILD_DIR/$PROJECT_NAME-linux-amd64" \
    .

echo "Building for linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-w -s" \
    -o "../$BUILD_DIR/$PROJECT_NAME-linux-arm64" \
    .

cd ..

# If VERSION is set, create checksums for release
if [ -n "$VERSION" ]; then
    echo "Creating checksums for release $VERSION..."
    cd "$BUILD_DIR"
    sha256sum * > checksums.txt
    echo "✓ Checksums created:"
    cat checksums.txt
    cd ..
fi

echo ""
echo "✓ Build complete!"
ls -lah "$BUILD_DIR"
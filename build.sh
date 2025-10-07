#!/bin/bash

set -e

# Build script for tailstream-agent

PROJECT_NAME="tailstream-agent"
BUILD_DIR="dist"

echo "Building $PROJECT_NAME..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Get build info
VERSION=${VERSION:-"dev"}
BUILD_DATE=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Version: $VERSION"
echo "Build Date: $BUILD_DATE"
echo "Git Commit: $GIT_COMMIT"

# Build for Linux x64
cd agent
echo "Building for linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GitCommit=$GIT_COMMIT'" \
    -o "../$BUILD_DIR/$PROJECT_NAME-linux-amd64" \
    .

echo "Building for linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-w -s -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GitCommit=$GIT_COMMIT'" \
    -o "../$BUILD_DIR/$PROJECT_NAME-linux-arm64" \
    .

echo "Building for darwin/amd64 (macOS Intel)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
    -ldflags="-w -s -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GitCommit=$GIT_COMMIT'" \
    -o "../$BUILD_DIR/$PROJECT_NAME-darwin-amd64" \
    .

echo "Building for darwin/arm64 (macOS Apple Silicon)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
    -ldflags="-w -s -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GitCommit=$GIT_COMMIT'" \
    -o "../$BUILD_DIR/$PROJECT_NAME-darwin-arm64" \
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
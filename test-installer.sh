#!/bin/bash

# Test script for the installer using Docker
set -e

echo "🧪 Testing Tailstream Agent Installer"
echo "======================================"

# Build the test image
echo "Building test Docker image..."
docker build -f Dockerfile.installer-test -t tailstream-installer-test .

echo ""
echo "Running installer test..."
echo "========================"

# Run the test - using --privileged to allow systemd operations
docker run --rm --privileged tailstream-installer-test

echo ""
echo "✅ Test completed"
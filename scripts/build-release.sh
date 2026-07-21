#!/bin/bash

set -e

RELEASE_VERSION="0.1.0"
RELEASE_DIR="dist/OpenAD-Linux-v${RELEASE_VERSION}"

echo "🚀 Starting production build..."

mkdir -p dist

# Build backend
echo "📦 Building backend..."
cd apps/backend
go mod tidy
go test ./...
go build -o ../../dist/openad-server ./cmd/api
go build -o ../../dist/openad-cli ./cmd/cli
cd ../..

# Build frontend
echo "🎨 Building frontend..."
cd apps/web
npm ci
npm run build:static
cp -r out ../../dist/web
cd ../..

# Create release package
echo "📋 Creating release package..."
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# Copy binaries
cp dist/openad-* "$RELEASE_DIR/"
cp -r dist/web "$RELEASE_DIR/"

# Copy documentation
cp README.md "$RELEASE_DIR/"
cp docs/RELEASE_NOTES.md "$RELEASE_DIR/"

# Create checksums
cd dist
sha256sum "OpenAD-Linux-v${RELEASE_VERSION}"/* > checksums.txt
cd ..

echo "OpenAD build completed successfully!"
echo "OpenAD release package: ${RELEASE_DIR}/"

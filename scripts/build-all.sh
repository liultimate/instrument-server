#!/bin/bash

APP="instrument-server"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0")

echo "🚀 构建 ${APP} ${VERSION}"

# Linux
echo "🔨 Linux amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/${APP}-linux-amd64

# Windows
echo "🔨 Windows amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/${APP}-windows-amd64.exe

# macOS
echo "🔨 macOS amd64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/${APP}-darwin-amd64

echo "🔨 macOS arm64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/${APP}-darwin-arm64

echo "✅ 完成!"
ls -lh dist/

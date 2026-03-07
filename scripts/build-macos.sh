#!/usr/bin/env bash
# OmniLLM-Studio — Wails Build Script for macOS
# Produces: build/bin/OmniLLM-Studio.app
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   - Xcode Command Line Tools: xcode-select --install
#
# Usage:
#   ./build-macos.sh                 # Build for current architecture
#   GOARCH=arm64 ./build-macos.sh    # Build for Apple Silicon
#   GOARCH=amd64 ./build-macos.sh    # Build for Intel

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default to current architecture
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"
PLATFORM="darwin/$TARGET_ARCH"

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Build for macOS"
echo "  Architecture: $TARGET_ARCH"
echo "=========================================="
echo ""

# --- Check prerequisites ---
for cmd in go node npm wails; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] '$cmd' not found in PATH."
        if [ "$cmd" = "wails" ]; then
            echo "  Install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
        fi
        exit 1
    fi
done

# --- Build frontend ---
echo "[1/3] Building frontend..."
cd "$PROJECT_ROOT/frontend"
npm ci --silent
npm run build

# --- Copy frontend dist to desktop embed directory ---
echo "[2/3] Embedding frontend assets..."
EMBED_DIR="$PROJECT_ROOT/backend/cmd/desktop/frontend_dist"
rm -rf "$EMBED_DIR"
cp -r "$PROJECT_ROOT/frontend/dist" "$EMBED_DIR"

# --- Build with Wails ---
echo "[3/3] Building macOS binary with Wails..."
cd "$PROJECT_ROOT/backend/cmd/desktop"

GIT_VERSION="$(git describe --tags --always 2>/dev/null || echo 'dev')"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

wails build -s -clean -trimpath -platform "$PLATFORM" \
    -ldflags "-X main.version=$GIT_VERSION -X main.commit=$GIT_COMMIT"

# Copy output to project build directory
mkdir -p "$PROJECT_ROOT/build/bin"
if [ -d "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio.app" ]; then
    rm -rf "$PROJECT_ROOT/build/bin/OmniLLM-Studio.app"
    cp -r "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio.app" \
        "$PROJECT_ROOT/build/bin/OmniLLM-Studio.app"
else
    cp -f "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio" \
        "$PROJECT_ROOT/build/bin/OmniLLM-Studio" 2>/dev/null || true
fi

echo ""
echo "=========================================="
echo "  Build complete!"
echo "  Output: build/bin/OmniLLM-Studio.app"
echo "=========================================="
echo ""
    <string>11.0</string>
</dict>
</plist>
PLIST

echo ""
echo "=========================================="
echo "  Build complete!"
echo "  Output: build/bin/OmniLLM-Studio.app"
echo "=========================================="
echo ""

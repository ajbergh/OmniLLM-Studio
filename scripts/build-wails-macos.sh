#!/usr/bin/env bash
# OmniLLM-Studio — Wails Build Script for macOS
# Produces: build/bin/OmniLLM-Studio[.app]
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   - Xcode Command Line Tools: xcode-select --install  (needed by Wails for WebKit/Cocoa — NOT for SQLite)
#
# Usage:
#   ./build-macos.sh              # Build for current architecture
#   ./build-macos.sh amd64        # Intel x86_64
#   ./build-macos.sh arm64        # Apple Silicon
#   ./build-macos.sh universal    # Universal binary (amd64 + arm64)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Architecture ---
RAW_ARCH="${1:-${GOARCH:-$(go env GOARCH 2>/dev/null || echo 'amd64')}}"
UNIVERSAL=false
case "$RAW_ARCH" in
    x64|x86_64|amd64)  TARGET_ARCH="amd64" ;;
    arm64|aarch64)     TARGET_ARCH="arm64" ;;
    universal)         TARGET_ARCH="universal"; UNIVERSAL=true ;;
    *)
        echo "[ERROR] Unsupported architecture: $RAW_ARCH (use amd64, arm64, or universal)"
        exit 1
        ;;
esac
PLATFORM="darwin/$TARGET_ARCH"

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Build for macOS"
echo "  Architecture: $TARGET_ARCH"
echo "=========================================="
echo ""

# --- Check prerequisites ---
# CGO is required on macOS for Wails WebKit/Cocoa bindings (not for SQLite — we use pure-Go modernc.org/sqlite)
export CGO_ENABLED=1

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
OUTPUT_SUFFIX=""
[ "$TARGET_ARCH" = "arm64" ]     && OUTPUT_SUFFIX="-arm64"
[ "$TARGET_ARCH" = "universal" ] && OUTPUT_SUFFIX="-universal"

mkdir -p "$PROJECT_ROOT/build/bin"
if [ -d "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio.app" ]; then
    rm -rf "$PROJECT_ROOT/build/bin/OmniLLM-Studio${OUTPUT_SUFFIX}.app"
    cp -r "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio.app" \
        "$PROJECT_ROOT/build/bin/OmniLLM-Studio${OUTPUT_SUFFIX}.app"
    echo ""
    echo "=========================================="
    echo "  Build complete!"
    echo "  Output: build/bin/OmniLLM-Studio${OUTPUT_SUFFIX}.app"
    echo "=========================================="
else
    cp -f "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio" \
        "$PROJECT_ROOT/build/bin/OmniLLM-Studio${OUTPUT_SUFFIX}" 2>/dev/null || true
    echo ""
    echo "=========================================="
    echo "  Build complete!"
    echo "  Output: build/bin/OmniLLM-Studio${OUTPUT_SUFFIX}"
    echo "=========================================="
fi
echo ""

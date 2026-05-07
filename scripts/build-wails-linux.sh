#!/usr/bin/env bash
# OmniLLM-Studio — Wails Build Script for Linux
# Produces: build/bin/OmniLLM-Studio[-arm64]
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   - GCC + pkg-config
#   - WebKit2GTK: sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev (Debian/Ubuntu)
#
# Usage:
#   ./build-linux.sh              # Linux x64 (amd64)
#   ./build-linux.sh arm64        # Linux ARM64

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Architecture ---
RAW_ARCH="${1:-${GOARCH:-amd64}}"
case "$RAW_ARCH" in
    x64|x86_64|amd64) TARGET_ARCH="amd64" ;;
    arm64|aarch64)    TARGET_ARCH="arm64" ;;
    *)
        echo "[ERROR] Unsupported architecture: $RAW_ARCH (use amd64 or arm64)"
        exit 1
        ;;
esac
PLATFORM="linux/$TARGET_ARCH"

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Build for Linux"
echo "  Architecture: $TARGET_ARCH"
echo "=========================================="
echo ""

# --- Check prerequisites ---
for cmd in go node npm wails gcc pkg-config; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] '$cmd' not found in PATH."
        if [ "$cmd" = "wails" ]; then
            echo "  Install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
        fi
        exit 1
    fi
done

# Check for WebKit2GTK headers
if ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
    echo "[ERROR] WebKit2GTK development headers not found."
    echo "  Debian/Ubuntu:  sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"
    echo "  Fedora:         sudo dnf install gtk3-devel webkit2gtk4.0-devel"
    echo "  Arch:           sudo pacman -S webkit2gtk"
    exit 1
fi

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
echo "[3/3] Building Linux binary with Wails..."
cd "$PROJECT_ROOT/backend/cmd/desktop"

GIT_VERSION="$(git describe --tags --always 2>/dev/null || echo 'dev')"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

wails build -s -clean -trimpath -platform "$PLATFORM" \
    -ldflags "-X main.version=$GIT_VERSION -X main.commit=$GIT_COMMIT"

# Copy output to project build directory
OUTPUT_NAME="OmniLLM-Studio"
[ "$TARGET_ARCH" = "arm64" ] && OUTPUT_NAME="OmniLLM-Studio-arm64"

mkdir -p "$PROJECT_ROOT/build/bin"
cp -f "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio" \
    "$PROJECT_ROOT/build/bin/$OUTPUT_NAME"

echo ""
echo "=========================================="
echo "  Build complete!"
echo "  Output: build/bin/$OUTPUT_NAME"
echo "=========================================="
echo ""

#!/usr/bin/env bash
# OmniLLM-Studio — Web Server Build for Linux (non-Wails)
# Produces: build/web/omnillm-studio[-arm64] + build/web/frontend/
#
# This builds the standalone Go web server and the React frontend.
# The server exposes the REST/SSE API on :8080.  Serve the frontend
# separately (nginx, Caddy, or `npx serve`).
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - GCC for CGO/SQLite
#
# Usage:
#   ./build-web-linux.sh              # Linux x64 (amd64)
#   ./build-web-linux.sh arm64        # Linux ARM64

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

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Web Build (Linux)"
echo "  Architecture: $TARGET_ARCH"
echo "=========================================="
echo ""

# --- Check prerequisites ---
for cmd in go node npm gcc; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "[ERROR] '$cmd' not found in PATH."
        exit 1
    fi
done

# --- Build frontend ---
echo "[1/3] Building frontend..."
cd "$PROJECT_ROOT/frontend"
npm ci --silent
npm run build

# --- Build Go web server ---
echo "[2/3] Building Go web server..."
cd "$PROJECT_ROOT/backend"

GIT_VERSION="$(git describe --tags --always 2>/dev/null || echo 'dev')"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

OUTPUT_NAME="omnillm-studio"
[ "$TARGET_ARCH" = "arm64" ] && OUTPUT_NAME="omnillm-studio-arm64"

mkdir -p "$PROJECT_ROOT/build/web"

CGO_ENABLED=1 GOOS=linux GOARCH="$TARGET_ARCH" \
    go build -trimpath \
        -ldflags "-s -w -X main.version=$GIT_VERSION -X main.commit=$GIT_COMMIT" \
        -o "$PROJECT_ROOT/build/web/$OUTPUT_NAME" \
        ./cmd/server

# --- Copy frontend dist ---
echo "[3/3] Copying frontend assets..."
WEB_FRONTEND="$PROJECT_ROOT/build/web/frontend"
rm -rf "$WEB_FRONTEND"
cp -r "$PROJECT_ROOT/frontend/dist" "$WEB_FRONTEND"

echo ""
echo "=========================================="
echo "  Web build complete!"
echo "  Server:   build/web/$OUTPUT_NAME"
echo "  Frontend: build/web/frontend/"
echo ""
echo "  Run:  ./build/web/$OUTPUT_NAME"
echo "  Serve frontend separately (nginx / Caddy /"
echo "    npx serve build/web/frontend)"
echo "=========================================="
echo ""

#!/usr/bin/env bash
# OmniLLM-Studio — Web Server Build for macOS (non-Wails)
# Produces: build/web/omnillm-studio[-arm64|-universal] + build/web/frontend/
#
# This builds the standalone Go web server and the React frontend.
# The server exposes the REST/SSE API on :8080.  Serve the frontend
# separately (nginx, Caddy, or `npx serve`).
#
# Requirements:
#   - Go 1.24+
#   - Node.js 18+
#   - Xcode Command Line Tools: xcode-select --install
#
# Usage:
#   ./build-web-macos.sh              # Current architecture
#   ./build-web-macos.sh amd64        # Intel x86_64
#   ./build-web-macos.sh arm64        # Apple Silicon
#   ./build-web-macos.sh universal    # Universal binary (amd64 + arm64)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Architecture ---
RAW_ARCH="${1:-${GOARCH:-$(go env GOARCH 2>/dev/null || echo 'amd64')}}"
UNIVERSAL=false
case "$RAW_ARCH" in
    x64|x86_64|amd64) TARGET_ARCH="amd64" ;;
    arm64|aarch64)    TARGET_ARCH="arm64" ;;
    universal)        TARGET_ARCH="universal"; UNIVERSAL=true ;;
    *)
        echo "[ERROR] Unsupported architecture: $RAW_ARCH (use amd64, arm64, or universal)"
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo "  OmniLLM-Studio — Web Build (macOS)"
echo "  Architecture: $TARGET_ARCH"
echo "=========================================="
echo ""

# --- Check prerequisites ---
for cmd in go node npm; do
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

mkdir -p "$PROJECT_ROOT/build/web"

build_binary() {
    local arch="$1"
    local out="$2"
    CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" \
        go build -trimpath \
            -ldflags "-s -w -X main.version=$GIT_VERSION -X main.commit=$GIT_COMMIT" \
            -o "$out" \
            ./cmd/server
}

if $UNIVERSAL; then
    echo "  Building amd64 slice..."
    build_binary "amd64" "$PROJECT_ROOT/build/web/omnillm-studio-amd64-tmp"
    echo "  Building arm64 slice..."
    build_binary "arm64" "$PROJECT_ROOT/build/web/omnillm-studio-arm64-tmp"
    echo "  Creating universal binary with lipo..."
    lipo -create -output "$PROJECT_ROOT/build/web/omnillm-studio-universal" \
        "$PROJECT_ROOT/build/web/omnillm-studio-amd64-tmp" \
        "$PROJECT_ROOT/build/web/omnillm-studio-arm64-tmp"
    rm -f "$PROJECT_ROOT/build/web/omnillm-studio-amd64-tmp" \
           "$PROJECT_ROOT/build/web/omnillm-studio-arm64-tmp"
    OUTPUT_NAME="omnillm-studio-universal"
else
    OUTPUT_SUFFIX=""
    [ "$TARGET_ARCH" = "arm64" ] && OUTPUT_SUFFIX="-arm64"
    OUTPUT_NAME="omnillm-studio${OUTPUT_SUFFIX}"
    build_binary "$TARGET_ARCH" "$PROJECT_ROOT/build/web/$OUTPUT_NAME"
fi

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

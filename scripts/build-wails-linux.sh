#!/usr/bin/env bash
# OmniLLM-Studio — Wails Linux build
# Produces: build/bin/OmniLLM-Studio[-arm64]
#
# Requirements:
#   - Go 1.25+
#   - Node.js 20+
#   - Wails CLI v2.12.0
#   - GCC + pkg-config
#   - WebKit2GTK 4.0, or WebKit2GTK 4.1 with the webkit2_41 build tag

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RAW_ARCH="${1:-${GOARCH:-amd64}}"
case "$RAW_ARCH" in
    x64|x86_64|amd64) TARGET_ARCH="amd64" ;;
    arm64|aarch64)    TARGET_ARCH="arm64" ;;
    *)
        echo "[ERROR] Unsupported architecture: $RAW_ARCH (use amd64 or arm64)" >&2
        exit 1
        ;;
esac
PLATFORM="linux/$TARGET_ARCH"

printf '\n==========================================\n'
printf '  OmniLLM-Studio — Build for Linux\n'
printf '  Architecture: %s\n' "$TARGET_ARCH"
printf '==========================================\n\n'

export CGO_ENABLED=1
for cmd in go node npm wails gcc pkg-config; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "[ERROR] '$cmd' not found in PATH." >&2
        [ "$cmd" = "wails" ] && echo "  Install with: go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0" >&2
        exit 1
    fi
done

WAILS_TAG_ARGS=()
if pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
    echo "[info] Using WebKit2GTK 4.0"
elif pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    echo "[info] Using WebKit2GTK 4.1"
    WAILS_TAG_ARGS=(-tags webkit2_41)
else
    echo "[ERROR] WebKit2GTK development headers not found." >&2
    echo "  Ubuntu 24.04+: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev" >&2
    echo "  Older Debian/Ubuntu: sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev" >&2
    exit 1
fi

printf '[1/3] Building frontend...\n'
cd "$PROJECT_ROOT/frontend"
npm ci --silent
npm run build

printf '[2/3] Embedding frontend assets...\n'
EMBED_DIR="$PROJECT_ROOT/backend/cmd/desktop/frontend_dist"
rm -rf "$EMBED_DIR"
cp -r "$PROJECT_ROOT/frontend/dist" "$EMBED_DIR"

printf '[3/3] Building Linux binary with Wails...\n'
cd "$PROJECT_ROOT/backend/cmd/desktop"
GIT_VERSION="$(git describe --tags --always 2>/dev/null || echo 'dev')"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

wails build -s -clean -trimpath -platform "$PLATFORM" \
    "${WAILS_TAG_ARGS[@]}" \
    -ldflags "-X main.version=$GIT_VERSION -X main.commit=$GIT_COMMIT"

OUTPUT_NAME="OmniLLM-Studio"
[ "$TARGET_ARCH" = "arm64" ] && OUTPUT_NAME="OmniLLM-Studio-arm64"
mkdir -p "$PROJECT_ROOT/build/bin"
cp -f "$PROJECT_ROOT/backend/cmd/desktop/build/bin/OmniLLM-Studio" \
    "$PROJECT_ROOT/build/bin/$OUTPUT_NAME"

printf '\n==========================================\n'
printf '  Build complete!\n'
printf '  Output: build/bin/%s\n' "$OUTPUT_NAME"
printf '==========================================\n'
